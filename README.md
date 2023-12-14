# liferay-devops-challenge

## Requirements

- Docker
- Golang 1.18
- Kubectl
- Helm
- Trivy
- Kind
- Linux binaries: envsubst

## Directory Structure

The project follows a well-organized structure:

- **applications:** Holds all applications.
- **automations:** Contains automations for streamlining the development lifecycle.
  - **deployer:** A Golang-based CLI for deploying, releasing, and testing applications, as well as managing Kubernetes vendors.
- **infrastructure:** Houses infrastructure components such as Kubernetes clusters, Helm charts, Vendors, etc.
- **ops:** Holds operations configurations for each application.

## Install

To install the deployment tool:

1.  Navigate to the `automations/deployer` directory.
2.  Run `go build` to generate the binary (deployer).
3.  Copy the binary to `~/.local/bin` or `/bin/`.
4.  Run `deployer -h` to verify the installation.

## Setup

Follow these steps to set up the environment:

1.  Clone the directory the home directory. The path should be: `$HOME/liferay-devops-challenge`.

2.  Source the `.env.example.sh` to load the necessary environment variables.

3.  Go to the `infrastructure/cluster` directory.

    - The local Kubernetes cluster configuration file is `kind-cluster.yaml`.
    - Make sure the `deploy.sh` script is executable.
    - Deploy the cluster using the provided `deploy.sh` script to configure the private registry for all nodes properly.
    - Authentication with the docker registry (Docker Hub) is required.
    - Run the following command:

    ```sh
        chmod +x deploy.sh
        ./deploy.sh -f /home/<user>/liferay-devops-challenge/infrastructure/cluster/kind-cluster.yaml -u <docker_username_for_the_private_registry>
    ```

    - The local Kubernetes cluster will be deployed with the `kind` tool.

4.  Deploy vendors to the local Kubernetes cluster.

    - Go to the `infrastructure` directory.
    - Run the `repositories.sh` script to update the Helm repositories.
    - The `infra.yaml` file contains a list of vendors to deploy.
    - Run the following command:

    ```sh
      deployer vendors deploy -e "production" -i "/home/<user>/liferay-devops-challenge/infrastructure"
    ```

5.  Release the application and push the Docker Image to the private registry.

    - Run the following command:

    ```sh
      deployer release -d "/home/<user>/liferay-devops-challenge/applications/typeorm-typescript-express-example" \
          -o "/home/<user>/liferay-devops-challenge/ops" -u "<docker_hub_username>"
    ```

    Trivy will scan the Docker image for vulnerabilities.

    Check the output for the release. The image will be pushed to the private registry.
    If successful, the version will be update on the `package.json` file (version bump),
    and the property `latestReleaseVersion` will be updated to the recently published version.
    This property is defined in the `ops/<application_name>/deploy.yaml` directory.

6.  Deploy the application to the local Kubernetes cluster.

    - Make sure to set the "environmentVars" defined in the `<ops_directory>/<application_directory>/deploy.yaml` file in your current shell session.
    - Change the `image.repo` defined in the `values.production.yaml` file for the application.
    - Run the following command:

    ```sh
      deployer deploy -d "/home/<user>/liferay-devops-challenge/applications/typeorm-typescript-express-example" \
        -o "/home/<user>/liferay-devops-challenge/ops" \
        -i "/home/<user>/liferay-devops-challenge/infrastructure" \
        -e "production"
    ```

    The are three worker nodes with different zone labels, the application by default has 3
    replicas, each pod will be scheduled on a different worker node based on the zone label.
    To ensure high availability.

    The namespace and the helm release name by default is the application name: typeorm-typescript-express-example

    The "-n" namespace and "-t" image tag are optional. Don't need to specify them.

7.  Test the application.

    - Run the following command:

    ```sh
        deployer test functional -d  "/home/<user>/liferay-devops-challenge/applications/typeorm-typescript-express-example" \
            -e "/backend/posts" \
            -u localhost
    ```

    Considering that the application is exposed by Nginx, the path to the application is
    `/backend`, however the request gets redirected to `/posts` internally,
    so the request will be processed by the application and return 200 OK.
    The body will be printed to the console.

8.  Don't forget to cleanup the resources created by the test.

If you have any questions, please contact me.

Thanks for reading,
Lucas Cicco.
