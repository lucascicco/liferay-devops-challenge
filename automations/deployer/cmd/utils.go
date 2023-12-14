package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// captureOutput reads from the provided reader and returns the content as a byte slice
func captureOutput(reader io.Reader) ([]byte, error) {
	var buf []byte
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		fmt.Println(string(line)) // Print the output in real-time
		buf = append(buf, append(line, '\n')...)
	}
	return buf, nil
}

// ExecuteCommand prints the command and then executes it
func ExecuteCommand(cmd *exec.Cmd) ([]byte, error) {
	fmt.Println("Running command:", strings.Join(cmd.Args, " "))

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stderr pipe: %v", err)
	}

	// Use WaitGroup to wait for the goroutines to finish
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to capture stdout
	var stdoutBuf []byte
	go func() {
		defer wg.Done()
		stdoutBuf, _ = captureOutput(stdout)
	}()

	// Goroutine to capture stderr
	var stderrBuf []byte
	go func() {
		defer wg.Done()
		stderrBuf, _ = captureOutput(stderr)
	}()

	// Start the command
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting command: %v", err)
	}

	// Wait for the command to finish and wait for goroutines to finish
	err = cmd.Wait()
	wg.Wait()

	if err != nil {
		return nil, fmt.Errorf("command failed: %v", err)
	}

	return append(stdoutBuf, stderrBuf...), nil
}

func CheckTargetEnvironment(env string) error {
	found := false
	for _, targetEnv := range []string{"development", "homolog", "production"} {
		if env == targetEnv {
			found = true
			break
		}
	}
	if !found {
		return errors.Errorf("invalid environment %s", env)
	}
	return nil
}

func ParseInfraConfigFromYaml(data []byte) (InfraConfig, error) {
	var infraConfig InfraConfig

	if err := yaml.Unmarshal(data, &infraConfig); err != nil {
		return infraConfig, errors.Wrap(err, "failed to unmarshal yaml file")
	}

	var wrongTypeFields []string
	for _, vendor := range infraConfig.Vendors.Charts {
		var missingFields []string
		if vendor.Name == "" {
			missingFields = append(missingFields, "name")
		}
		if vendor.Chart == "" {
			missingFields = append(missingFields, "chart")
		}
		if vendor.Namespace == "" {
			missingFields = append(missingFields, "namespace")
		}
		if vendor.ReleaseName == "" {
			missingFields = append(missingFields, "releaseName")
		}
		wrongTypeFields = append(wrongTypeFields, fmt.Sprintf("%s: %s", vendor, strings.Join(missingFields, ", ")))
	}

	for _, vendor := range infraConfig.Vendors.Scripts {
		if vendor == "" {
			wrongTypeFields = append(wrongTypeFields, fmt.Sprintf("script: %s", "empty"))
		}
	}

	if len(wrongTypeFields) > 0 {
		return infraConfig, errors.Wrapf(nil, "wrong type fields: %s", strings.Join(wrongTypeFields, ", "))
	}

	return infraConfig, nil
}

func GetMapFromYamlFile(yamlFile string) (map[string]interface{}, error) {
	file, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read yaml file %s", yamlFile)
	}
	var yamlFileData map[string]interface{}
	if err := yaml.Unmarshal(file, &yamlFileData); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal yaml file %s", yamlFile)
	}
	return yamlFileData, nil
}

// GetFieldsFromPackageJSON extracts specified fields from package.json.
func GetFieldsFromYamlFile(yamlFile string, fields []string) (map[string]interface{}, error) {
	data, err := GetMapFromYamlFile(yamlFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get yaml data")
	}

	newData := make(map[string]interface{})
	missingFields := []string{}

	for _, field := range fields {
		value, ok := data[field]
		if !ok {
			missingFields = append(missingFields, field)
		} else {
			newData[field] = value
		}
	}

	if len(missingFields) > 0 {
		return newData, fmt.Errorf("the following fields are missing from yaml file: %s", strings.Join(missingFields, ", "))
	}

	return newData, nil
}

// GetMapFromPackageJSON reads and unmarshals the content of package.json into a map.
func GetMapFromPackageJSON(appDir string) (map[string]interface{}, error) {
	packageJSONPath := filepath.Join(appDir, "package.json")
	file, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read package.json")
	}
	var data map[string]interface{}
	if err := json.Unmarshal(file, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal package.json")
	}
	return data, nil
}

// GetFieldsFromPackageJSON extracts specified fields from package.json.
func GetFieldsFromPackageJSON(appDir string, fields []string) (map[string]interface{}, error) {
	data, err := GetMapFromPackageJSON(appDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get package.json data")
	}

	newData := make(map[string]interface{})
	missingFields := []string{}

	for _, field := range fields {
		value, ok := data[field]
		if !ok {
			missingFields = append(missingFields, field)
		} else {
			newData[field] = value
		}
	}

	if len(missingFields) > 0 {
		return newData, fmt.Errorf("the following fields are missing from package.json: %s", strings.Join(missingFields, ", "))
	}

	return newData, nil
}

// CheckDockerHubTagExistence checks if the Docker image tag exists on DockerHub.
func CheckDockerHubTagExistence(repo string, version string, username string, token string) (bool, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/%s", repo, version)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, errors.Wrap(err, "failed to create HTTP request")
	}
	req.SetBasicAuth(username, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, errors.Wrap(err, "failed to check DockerHub tag existence")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

// BuildDockerImage builds a Docker image for the given application directory.
func BuildDockerImage(appDir string, imageName string) error {
	dockerfilePath := filepath.Join(appDir, "Dockerfile")
	cmd := exec.Command("docker", "build", "-t", imageName, "-f", dockerfilePath, appDir)
	_, err := ExecuteCommand(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to build Docker image")
	}
	return nil
}

// RunTrivy scans the Docker image for security vulnerabilities using Trivy.
func RunTrivy(imageName string) error {
	cmd := exec.Command("trivy", "image", "--severity", "HIGH,CRITICAL", "--exit-code", "1", imageName)
	_, err := ExecuteCommand(cmd)
	if err != nil {
		return errors.Wrap(err, "Trivy found security vulnerabilities")
	}
	return nil
}

// PushDockerImage pushes the Docker image to the specified repository.
func PushDockerImage(imageName string) error {
	cmd := exec.Command("docker", "push", imageName)
	_, err := ExecuteCommand(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to push Docker image")
	}
	return nil
}

// BumpPackageJSONVersion increments the patch version in package.json.
func BumpPackageJSONVersion(appDir string) error {
	packageJSONPath := filepath.Join(appDir, "package.json")

	// Read the current version from package.json
	jsonData, err := GetMapFromPackageJSON(appDir)
	if err != nil {
		return errors.Wrap(err, "failed to read package.json version")
	}

	version, ok := jsonData["version"].(string)
	if !ok {
		return errors.New("version is not a string")
	}

	// Bump the version (e.g., incrementing the patch version)
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return errors.New("invalid version format")
	}

	// Increment the patch version
	patchVersion := parts[2]
	patchVersionNumber := 0
	fmt.Sscanf(patchVersion, "%d", &patchVersionNumber)
	patchVersionNumber++
	parts[2] = fmt.Sprintf("%d", patchVersionNumber)

	newVersion := strings.Join(parts, ".")
	jsonData["version"] = newVersion

	updatedFile, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal package.json")
	}
	if err := os.WriteFile(packageJSONPath, updatedFile, 0644); err != nil {
		return errors.Wrap(err, "failed to write package.json")
	}
	log.Printf("Version bumped to %s\n", newVersion)
	return nil
}

func CheckIfPathExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}
	return nil
}

func ChownFileToCurrentUser(filename string) error {
	currentUser, err := user.Current()
	if err != nil {
		return errors.Wrap(err, "Failed to get current user information")
	}
	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		return errors.Wrap(err, "Failed to convert UID to integer")
	}
	gid, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		return errors.Wrap(err, "Failed to convert GID to integer")
	}
	return os.Chown(filename, uid, gid)
}

func MaskSensitiveData(data string, environment string) string {
	if environment == "production" {
		return "******"
	}
	return data
}

func ContainsKey(jsonData []byte, key string) bool {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		log.Printf("Error unmarshaling JSON data: %v\n", err)
		return false
	}

	_, exists := data[key]
	return exists
}
