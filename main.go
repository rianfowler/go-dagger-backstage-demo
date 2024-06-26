package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"dagger.io/dagger"
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

	// Clone the GitHub repository
	repo := client.Git(repoURL).Branch("main").Tree()

	// Sync the repository
	dir, err := repo.Sync(ctx)
	if err != nil {
		fmt.Println("Failed to sync repository:", err)
		return
	}

	// Define the build pack container
	buildContainer := client.Container().
		From("paketobuildpacks/builder:base").
		WithMountedDirectory("/workspace", dir).
		WithWorkdir("/workspace").
		WithExec([]string{"pack", "build", "my-app"})

	// Check the build result
	buildResult, err := buildContainer.Sync(ctx)
	if err != nil {
		fmt.Println("Build failed:", err)
		return
	}

	// Tag the built image
	imageName := "dagger-backstage-demo:latest"
	tarPath := "./dagger-backstage-demo.tar"
	_, err = buildResult.Export(ctx, tarPath)
	if err != nil {
		fmt.Println("Failed to export image:", err)
		return
	}
	if err != nil {
		fmt.Println("Failed to export image:", err)
		return
	}

	// Load the image into the local Docker daemon
	err = loadImageToLocalDocker(ctx, tarPath)
	if err != nil {
		fmt.Println("Failed to load image into local Docker daemon:", err)
		return
	}

	fmt.Println("Image published successfully:", imageName)
}

// loadImageToLocalDocker loads the image from a tar file into the local Docker daemon
func loadImageToLocalDocker(ctx context.Context, tarPath string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	tarFile, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	imageLoadResponse, err := cli.ImageLoad(ctx, tarFile, true)
	if err != nil {
		return err
	}
	defer imageLoadResponse.Body.Close()

	// Read the response
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, imageLoadResponse.Body)
	if err != nil {
		return err
	}

	fmt.Println(buf.String())
	return nil
}
