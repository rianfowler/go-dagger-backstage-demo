package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/dagger/dagger"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func main() {
	ctx := context.Background()

	// Connect to Dagger
	client, err := dagger.Connect(ctx)
	if err != nil {
		fmt.Println("Failed to connect to Dagger:", err)
		return
	}
	defer client.Close()

	// Define the repository and build pack
	repoURL := "https://github.com/rianfowler/backstage-dagger-demo.git"
	buildPack := "heroku/buildpacks:18"

	// Clone the GitHub repository
	repo := client.Git(repoURL).WithDir("/workspace")

	// Define the build pack container
	buildContainer := client.Container().
		From(buildPack).
		WithMountedDirectory("/workspace", repo)

	// Build the project
	buildResult := buildContainer.Exec(dagger.ContainerExecOpts{
		Args: []string{"npm", "install", "--prefix", "/workspace"},
	})

	if buildResult.ExitCode != 0 {
		fmt.Println("Build failed with exit code:", buildResult.ExitCode)
		return
	}

	// Tag the built image
	imageName := "dagger-backstage-demo:latest"
	pushResult := buildContainer.Publish(ctx, imageName)
	if pushResult != nil {
		fmt.Println("Failed to publish image:", pushResult)
		return
	}

	// Push to local Docker host
	err = pushToLocalDocker(ctx, imageName)
	if err != nil {
		fmt.Println("Failed to push image to local Docker host:", err)
		return
	}

	fmt.Println("Image published successfully:", imageName)
}

// pushToLocalDocker pushes the image to the local Docker host
func pushToLocalDocker(ctx context.Context, imageName string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	out, err := cli.ImagePush(ctx, imageName, types.ImagePushOptions{RegistryAuth: "unused"})
	if err != nil {
		return err
	}
	defer out.Close()

	// Read the output
	buf := new(bytes.Buffer)
	buf.ReadFrom(out)
	fmt.Println(buf.String())

	return nil
}
