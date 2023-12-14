package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	username string
	token    string
)

func init() {
	rootCmd.AddCommand(releaseCmd)

	releaseCmd.Flags().StringVarP(&appDir, "application_directory", "d", "", "the path of the application directory")
	releaseCmd.MarkFlagRequired("application_directory")

	releaseCmd.Flags().StringVarP(&opsDir, "operations_directory", "o", "", "the path of the operations directory")
	releaseCmd.MarkFlagRequired("operations_directory")

	releaseCmd.Flags().StringVarP(&username, "username", "u", os.Getenv("DOCKER_USERNAME"), "the username to access the private repository")
	releaseCmd.MarkFlagRequired("username")
}

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release the application",
	Run: func(cmd *cobra.Command, args []string) {
		token := os.Getenv("DOCKER_PASSWORD")
		if token == "" {
			log.Print("Insert the docker password/token below: \n")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			err := scanner.Err()
			if err != nil {
				log.Fatal(err)
			}
			token = scanner.Text()
		} else {
			log.Printf("Using Docker token from environment variable DOCKER_PASSWORD\n")
		}
		if err := runRelease(appDir, username, token, opsDir); err != nil {
			log.Fatalf("Error running release process: %v\n", err)
			os.Exit(1)
		}
	},
}

func runRelease(
	appDir string,
	username string,
	token string,
	opsDir string,
) error {
	log.Printf("Starting release process for application in directory: %s\n", appDir)

	log.Printf("Checking if the application directory exists: %s\n", appDir)
	if err := CheckIfPathExists(appDir); err != nil {
		return errors.Wrapf(err, "Directory %s does not exist", appDir)
	}

	var missingFields []string
	for _, val := range []string{username, token} {
		if val == "" {
			missingFields = append(missingFields, val)
		}
	}
	if len(missingFields) > 0 {
		return errors.Errorf("Missing required fields: %s", strings.Join(missingFields, ", "))
	}

	jsonData, err := GetFieldsFromPackageJSON(appDir, []string{"name", "version"})
	if err != nil {
		return err
	}
	version := jsonData["version"].(string)
	appName := jsonData["name"].(string)
	log.Printf("Read package.json: name=%s, version=%s\n", appName, version)

	// Check if the image tag already exists on DockerHub private repository
	log.Printf("Checking if image tag '%s' already exists on DockerHub private repository\n", version)
	imageRepo := fmt.Sprintf("%s/%s", username, appName)
	imageName := fmt.Sprintf("%s:%s", imageRepo, version)
	exists, err := CheckDockerHubTagExistence(imageRepo, version, username, token)
	if err != nil {
		return err
	}
	if exists {
		return errors.Errorf("Image tag '%s' already exists on DockerHub private repository", version)
	}
	// Build the Docker image
	log.Printf("Building Docker image for application: %s\n", appName)
	err = BuildDockerImage(appDir, imageName)
	if err != nil {
		return err
	}
	log.Printf("Built Docker image: %s\n", imageName)

	err = RunTrivy(imageName)
	if err != nil {
		return err
	}
	log.Printf("Ran Trivy for security checks on Docker image: %s\n", imageName)

	// Push the Docker image to the private repository
	err = PushDockerImage(imageName)
	if err != nil {
		return err
	}
	log.Printf("Pushed Docker image to the private repository: %s\n", imageName)

	deployFile := filepath.Join(opsDir, appName, "deploy.yaml")
	if err := CheckIfPathExists(deployFile); err != nil {
		return errors.Wrapf(err, "File %s does not exist", deployFile)
	}
	yamlData, err := GetMapFromYamlFile(deployFile)
	if err != nil {
		return errors.Wrapf(err, "Error reading YAML file %s", deployFile)
	}
	yamlData["latestReleaseVersion"] = version
	newYamlData, err := yaml.Marshal(yamlData)
	if err := os.WriteFile(deployFile, newYamlData, 0644); err != nil {
		return errors.Wrapf(err, "Error writing YAML file %s", deployFile)
	}
	log.Printf("Updated %s with latestReleaseVersion=%s\n", deployFile, version)

	// Bump the version in package.json
	err = BumpPackageJSONVersion(appDir)
	if err != nil {
		return err
	}
	log.Printf("Release process completed for version %s\n", version)
	return nil
}
