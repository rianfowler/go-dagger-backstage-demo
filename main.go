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

	// Define the repository
	repoURL := "https://github.com/rianfowler/backstage-dagger-demo.git"

	// Clone the GitHub repository
	repo := client.Git(repoURL).Branch("main").Tree()

	// Sync the repository
	dir, err := repo.Sync(ctx)
	if err != nil {
		fmt.Println("Failed to sync repository:", err)
		return
	}

	// Define the buildpack container using the latest Paketo Buildpacks for Node.js
	phases := []struct {
		name string
		args []string
	}{
		{"detector", []string{"/cnb/lifecycle/detector", "-app", "/workspace"}},
		{"analyzer", []string{"/cnb/lifecycle/analyzer", "-layers", "/layers", "-app", "/workspace"}},
		{"restorer", []string{"/cnb/lifecycle/restorer", "-layers", "/layers", "-app", "/workspace"}},
		{"builder", []string{"/cnb/lifecycle/builder", "-layers", "/layers", "-app", "/workspace"}},
		{"exporter", []string{"/cnb/lifecycle/exporter", "-layers", "/layers", "-app", "/workspace"}},
	}

	for _, phase := range phases {
		fmt.Printf("Running %s phase...\n", phase.name)
		buildContainer := client.Container().
			From("paketobuildpacks/builder-jammy-full").
			WithMountedDirectory("/workspace", dir).
			WithWorkdir("/workspace").
			WithoutUser(). // Run as root user
			WithEnvVariable("CNB_STACK_ID", "paketo-buildpacks/jammy").
			WithEnvVariable("CNB_PLATFORM_API", "0.3"). // Set the platform API version
			WithExec(phase.args)

		_, err := buildContainer.Sync(ctx)
		if err != nil {
			fmt.Printf("Build failed during %s phase: %v\n", phase.name, err)
			return
		}
	}
	// Define the buildpack container using a builder image that includes lifecycle binaries
	// buildContainer := client.Container().
	// 	From("paketobuildpacks/builder-jammy-full").
	// 	WithMountedDirectory("/workspace", dir).
	// 	WithWorkdir("/workspace").
	// 	// WithoutUser(). // Run as root user
	// 	//WithEnvVariable("CNB_STACK_ID", "paketo-buildpacks/jammy").
	// 	WithEnvVariable("CNB_PLATFORM_API", "0.5"). // Set the platform API version
	// 	WithExec([]string{"/cnb/lifecycle/creator", "-app", "/workspace"})
	// Check the build result
	// buildResult, err := buildContainer.Sync(ctx)
	// if err != nil {
	// 	fmt.Println("Build failed:", err)
	// 	return
	// }

	// Export the built image to a tarball
	tarPath := "./dagger-backstage-demo.tar"
	exportContainer := client.Container().
		From("paketobuildpacks/builder-jammy-full").
		WithMountedDirectory("/workspace", dir).
		WithWorkdir("/workspace").
		WithoutUser(). // Run as root user
		WithExec([]string{"/cnb/lifecycle/exporter", "-layers", "/layers", "-app", "/workspace"})

	exportResult, err := exportContainer.Sync(ctx)
	if err != nil {
		fmt.Println("Failed to export image:", err)
		return
	}

	_, err = exportResult.Export(ctx, tarPath)
	if err != nil {
		fmt.Println("Failed to export image:", err)
		return
	}
	// _, err = buildResult.Export(ctx, tarPath)
	// if err != nil {
	// 	fmt.Println("Failed to export image:", err)
	// 	return
	// }

	// Load the image into the local Docker daemon
	err = loadImageToLocalDocker(ctx, tarPath)
	if err != nil {
		fmt.Println("Failed to load image into local Docker daemon:", err)
		return
	}

	fmt.Println("Image built and loaded into local Docker successfully")
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
