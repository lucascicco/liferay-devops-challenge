package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	imageTag          string
	targetEnvironment string
)

func init() {
	rootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringVarP(&appDir, "application_directory", "d", "", "the path of the application directory")
	deployCmd.MarkFlagRequired("application_directory")

	deployCmd.Flags().StringVarP(&opsDir, "operations_directory", "o", "", "the path of the operations directory")
	deployCmd.MarkFlagRequired("operations_directory")

	deployCmd.Flags().StringVarP(&infrastructureDir, "infrastructure_directory", "i", "", "the path of the infrastructure directory")
	deployCmd.MarkFlagRequired("infrastructure_directory")

	deployCmd.Flags().StringVarP(&targetEnvironment, "target_environment", "e", "", "the environment to deploy to")
	deployCmd.MarkFlagRequired("target_environment")

	// Optional
	deployCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "the namespace to deploy to")
	deployCmd.Flags().StringVarP(&imageTag, "image_tag", "t", "", "the image tag to use")
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the application",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDeploy(
			appDir,
			opsDir,
			infrastructureDir,
			targetEnvironment,
			namespace,
			imageTag,
		); err != nil {
			log.Fatalf("Error running deploy process: %v\n", err)
			os.Exit(1)
		}
	},
}

func runDeploy(
	appDir string,
	opsDir string,
	infrastructureDir string,
	environment string,
	namespace string,
	imageTag string,
) error {
	log.Printf("Checking if the application directory exists: %s\n", appDir)
	if err := CheckIfPathExists(appDir); err != nil {
		return errors.Wrapf(err, "Directory %s does not exist", appDir)
	}

	jsonData, err := GetFieldsFromPackageJSON(appDir, []string{"name"})
	if err != nil {
		return err
	}
	appName := jsonData["name"].(string)
	log.Printf("Read package.json: name=%s\n", appName)

	var deployFile string
	deployFile = filepath.Join(opsDir, appName, "deploy.yaml")
	if err := CheckIfPathExists(deployFile); err != nil {
		return errors.Wrapf(err, "File %s does not exist", deployFile)
	}

	deployFile = filepath.Join(opsDir, appName, "deploy.yaml")
	yamlData, err := GetFieldsFromYamlFile(deployFile, []string{
		"chart",
		"environmentVars",
		"latestReleaseVersion",
	})
	var environmentVars []string
	if envVarsInterface, ok := yamlData["environmentVars"].([]interface{}); ok {
		for _, v := range envVarsInterface {
			if s, ok := v.(string); ok {
				environmentVars = append(environmentVars, s)
			} else {
				log.Printf("Unexpected type found in environmentVars: %T", v)
			}
		}
	}
	releaseVersion, ok := yamlData["latestReleaseVersion"].(string)
	if !ok {
		return fmt.Errorf("latestReleaseVersion is not a string")
	}
	if imageTag != "" {
		releaseVersion = imageTag
	}
	environmentVars = append(environmentVars, "IMAGE_TAG")
	os.Setenv("IMAGE_TAG", releaseVersion)

	chartValues := filepath.Join(opsDir, appName, fmt.Sprintf("values.%s.yaml", environment))
	if err := CheckIfPathExists(chartValues); err != nil {
		return errors.Wrapf(err, "Values %s for the chart does not exist", chartValues)
	}

	chartDir := filepath.Join(infrastructureDir, "charts", yamlData["chart"].(string))
	if err := CheckIfPathExists(chartDir); err != nil {
		return errors.Wrapf(err, "Chart %s does not exist", chartDir)
	}

	valuesTmpFile := filepath.Join("/dev/shm", fmt.Sprintf("%s.yaml", uuid.New()))
	defer os.Remove(valuesTmpFile)

	bytesRead, err := os.ReadFile(chartValues)
	if err != nil {
		return errors.Wrapf(err, "Error reading file %s", chartValues)
	}
	// NOTE: Only the original file owner can read/write the file
	if err := os.WriteFile(valuesTmpFile, bytesRead, 0600); err != nil {
		return errors.Wrapf(err, "Error writing file %s", valuesTmpFile)
	}
	if err := ChownFileToCurrentUser(valuesTmpFile); err != nil {
		return errors.Wrapf(err, "Error changing ownership of file %s", valuesTmpFile)
	}

	var envErrors []string
	for _, envVar := range environmentVars {
		envValue, ok := os.LookupEnv(envVar)
		if envVar == "" || !ok {
			envErrors = append(envErrors, fmt.Sprintf("Environment variable %s not set", envVar))
			continue
		}
		log.Printf("Setting environment variable %s=%s\n", envVar, MaskSensitiveData(envValue, environment))
		cmd := exec.Command("sed", "-i", fmt.Sprintf("s/<%s>/%s/g", envVar, envValue), valuesTmpFile)
		_, err := cmd.CombinedOutput()
		if err != nil {
			envErrors = append(
				envErrors,
				fmt.Sprintf("Error setting environment variable %s: %s", envVar, err.Error()),
			)
			continue
		}
		log.Printf("Environment variable %s set\n", envVar)
	}

	if len(envErrors) > 0 {
		return errors.Errorf("Error setting environment variables: %s", strings.Join(envErrors, "\n"))
	}

	if namespace == "" {
		namespace = appName
	}

	log.Printf("Deploying application %s to namespace %s\n", appName, namespace)
	helmCmd := exec.Command(
		"helm",
		"upgrade",
		"--install",
		appName,
		chartDir,
		"--values",
		valuesTmpFile,
		"--namespace",
		namespace,
		"--create-namespace",
		"--wait",
	)
	out, err := ExecuteCommand(helmCmd)
	if err != nil {
		return errors.Wrapf(err, "Error running helm upgrade: %s", out)
	}
	log.Printf("Application %s deployed\n", appName)
	return nil
}
