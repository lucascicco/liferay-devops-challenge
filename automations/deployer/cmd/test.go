package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
)

var (
	endpoint string
	host     string
)

func init() {
	rootCmd.AddCommand()
	rootCmd.AddCommand(testCmd)
	testCmd.AddCommand(functionalTestCmd)

	functionalTestCmd.Flags().StringVarP(&appDir, "application_directory", "d", "", "the path of the application directory")
	functionalTestCmd.MarkFlagRequired("application_directory")

	functionalTestCmd.Flags().StringVarP(&host, "host", "u", "", "the host of the application")
	functionalTestCmd.MarkFlagRequired("host")

	functionalTestCmd.Flags().StringVarP(&endpoint, "endpoint", "e", "", "the endpoint of the application")
	functionalTestCmd.MarkFlagRequired("endpoint")
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test the application",
	Run:   func(cmd *cobra.Command, args []string) {},
}

var functionalTestCmd = &cobra.Command{
	Use:              "functional",
	Short:            "Run the functional test",
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runFunctionalTest(appDir, host, endpoint); err != nil {
			log.Fatalf("Error running functional test: %v", err)
			os.Exit(1)
		}
	},
}

func runFunctionalTest(appDir string, host string, endpoint string) error {
	log.Printf("Checking if the application directory exists: %s\n", appDir)
	if err := CheckIfPathExists(appDir); err != nil {
		return errors.Wrapf(err, "Directory %s does not exist", appDir)
	}
	jsonData, err := GetFieldsFromPackageJSON(appDir, []string{"name"})
	if err != nil {
		return err
	}
	appName := jsonData["name"].(string)
	log.Printf("Testing application: %s\n", appName)
	hosts := []string{}
	hosts = append(hosts, host)
	var failedHosts []string
	for _, host := range hosts {
		url := fmt.Sprintf("http://%s/%s", host, endpoint)
		fmt.Printf("Testing endpoint: %s\n", url)

		req, err := http.NewRequest("GET", url, nil)
		req.Header.Add("Accept", "application/json")
		if err != nil {
			log.Fatalf("Error creating HTTP request: %v\n", err)
			failedHosts = append(failedHosts, host)
			continue
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Error sending HTTP request: %v\n", err)
			failedHosts = append(failedHosts, host)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Non-200 status code: %d\n", resp.StatusCode)
			failedHosts = append(failedHosts, host)
			continue
		}
		body, err := httputil.DumpResponse(resp, true)
		if err != nil {
			log.Fatalf("Error dumping HTTP response: %v\n", err)
			failedHosts = append(failedHosts, host)
			continue
		}
		jsonData, err := json.MarshalIndent(string(body), "", "  ")
		if err != nil {
			log.Printf("Error marshaling JSON data: %v\n", err)
			failedHosts = append(failedHosts, host)
			continue
		}
		log.Printf("Response: %s\n", jsonData)
	}

	if len(failedHosts) > 0 {
		return fmt.Errorf(
			"Failed to test the following hosts: %s",
			strings.Join(failedHosts, ", "),
		)
	}

	log.Printf("All tests passed!\n")
	return nil
}
