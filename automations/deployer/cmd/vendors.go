package cmd

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type VendorChartConfig struct {
	Name        string   `yaml:"name"`
	Chart       string   `yaml:"chart"`
	Namespace   string   `yaml:"namespace"`
	ReleaseName string   `yaml:"releaseName"`
	Envs        []string `yaml:"envs"`
}

type VendorConfig struct {
	Scripts []string            `yaml:"scripts"`
	Charts  []VendorChartConfig `yaml:"charts"`
}

type InfraConfig struct {
	Vendors VendorConfig `yaml:"vendors"`
}

func init() {
	vendorsDeployCmd.Flags().StringVarP(
		&infrastructureDir,
		"infrastructure_directory",
		"i",
		"",
		"the path of the infrastructure directory",
	)
	vendorsDeployCmd.Flags().StringVarP(&targetEnvironment, "target_environment", "e", "", "the environment to deploy to")

	vendorsDeployCmd.MarkFlagRequired("target_environment")
	vendorsDeployCmd.MarkFlagRequired("infrastructure_directory")

	vendorsCmd.AddCommand(vendorsDeployCmd)
	rootCmd.AddCommand(vendorsCmd)
}

var vendorsCmd = &cobra.Command{
	Use: "vendors",
	Run: func(cmd *cobra.Command, args []string) {},
}

var vendorsDeployCmd = &cobra.Command{
	Use:              "deploy",
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runVendorsDeploy(infrastructureDir, targetEnvironment); err != nil {
			log.Fatalf("Error running vendors deploy: %v", err)
			os.Exit(1)
		}
	},
}

func runVendorsDeploy(
	infrastructureDir string,
	targetEnvironment string,
) error {
	log.Printf("Running vendors deploy for environment %s\n", targetEnvironment)
	if err := CheckTargetEnvironment(targetEnvironment); err != nil {
		return err
	}
	if err := CheckIfPathExists(infrastructureDir); err != nil {
		return errors.Wrapf(err, "Infrastructure directory %s does not exist", infrastructureDir)
	}
	infraConfigFile := filepath.Join(infrastructureDir, "infra.yaml")
	if err := CheckIfPathExists(infraConfigFile); err != nil {
		return errors.Wrapf(err, "Infrastructure file %s does not exist", infraConfigFile)
	}
	infraConfigData, err := os.ReadFile(infraConfigFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to read file %s", infraConfigFile)
	}
	infraConfig, err := ParseInfraConfigFromYaml(infraConfigData)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse vendors config from file %s", infraConfigFile)
	}
	vendorsDir := filepath.Join(infrastructureDir, "vendors")
	for _, vendor := range infraConfig.Vendors.Scripts {
		log.Printf("Deploying vendor %s\n", vendor)
		vendorDir := filepath.Join(vendorsDir, strings.ToLower(vendor))
		log.Printf("Vendor directory %s\n", vendorDir)
		if err := CheckIfPathExists(vendorDir); err != nil {
			return errors.Wrapf(err, "Vendor directory %s does not exist", vendorDir)
		}
		deployScript := filepath.Join(vendorDir, fmt.Sprintf("deploy.%s.sh", targetEnvironment))
		if err := CheckIfPathExists(deployScript); err != nil {
			return errors.Wrapf(err, "Deploy script %s does not exist", deployScript)
		}
		deployCmd := exec.Command(deployScript)
		_, err := ExecuteCommand(deployCmd)
		if err != nil {
			log.Fatalf("Error running deploy script %s: %v", deployScript, err)
			continue
		}
		log.Printf("Deployed script %s \n", deployScript)
	}

	for _, vendor := range infraConfig.Vendors.Charts {
		log.Printf("Deploying vendor %s\n", vendor.Name)

		vendorDir := filepath.Join(vendorsDir, strings.ToLower(vendor.Name))
		log.Printf("Vendor directory %s\n", vendorDir)
		if err := CheckIfPathExists(vendorDir); err != nil {
			return errors.Wrapf(err, "Vendor directory %s does not exist", vendorDir)
		}

		helmValuesFile := filepath.Join(
			vendorDir,
			fmt.Sprintf("values.%s.yaml", targetEnvironment),
		)
		if err := CheckIfPathExists(helmValuesFile); err != nil {
			return errors.Wrapf(err, "Helm values file %s does not exist", helmValuesFile)
		}

		valuesTmpFile := filepath.Join("/dev/shm", fmt.Sprintf("%s.yaml", uuid.New()))
		bytesRead, err := os.ReadFile(helmValuesFile)
		if err != nil {
			return errors.Wrapf(err, "Error reading file %s", helmValuesFile)
		}
		// NOTE: Only the original file owner can read/write the file
		if err := os.WriteFile(valuesTmpFile, bytesRead, 0600); err != nil {
			return errors.Wrapf(err, "Error writing file %s", valuesTmpFile)
		}
		if err := ChownFileToCurrentUser(valuesTmpFile); err != nil {
			return errors.Wrapf(err, "Error changing ownership of file %s", valuesTmpFile)
		}

		var envErrors []string
		for _, envVar := range vendor.Envs {
			envValue, ok := os.LookupEnv(envVar)
			if envVar == "" || !ok {
				envErrors = append(envErrors, fmt.Sprintf("Environment variable %s not set", envVar))
				continue
			}
			log.Printf(
				"Setting environment variable %s=%s\n",
				envVar,
				MaskSensitiveData(envValue, targetEnvironment),
			)
			cmd := exec.Command(
				"sed",
				"-i",
				fmt.Sprintf("s/<%s>/%s/g", envVar, envValue),
				valuesTmpFile,
			)
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
			return errors.Errorf(
				"Error setting environment variables: %s",
				strings.Join(envErrors, "\n"),
			)
		}

		log.Printf("Deploying vendor %s to namespace %s\n", vendor.Name, namespace)
		helmCmd := exec.Command(
			"helm",
			"upgrade",
			"--install",
			vendor.ReleaseName,
			vendor.Chart,
			"--values",
			valuesTmpFile,
			"--namespace",
			vendor.Namespace,
			"--create-namespace",
			"--wait",
		)
		_, err = ExecuteCommand(helmCmd)
		if err != nil {
			log.Fatalf("Error running deploy script %s: %v", helmCmd, err)
			continue
		}
		log.Printf("Vendor %s deployed\n", vendor.Name)
		os.Remove(valuesTmpFile)
	}
	return nil
}
