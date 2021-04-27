# Keptn Locust Service

This service provides a way to performance test on your application triggered by [Keptn](https://keptn.sh) using the [Locust](https://locust.io/) performance testing tool.

## Compatibility Matrix

| Keptn Version    | [locust-service Docker Image]() |
|:----------------:|:----------------------------------------:|
|       0.8.0      | to be added |

## Keptn CloudEvents

This service reacts on the following Keptn CloudEvents (see [deploy/service.yaml](deploy/service.yaml)):
* `sh.keptn.event.test.triggered` (used to be `sh.keptn.events.deployment-finished`) -> start locust performance tests
* `sh.keptn.event.test.finished` (used to be`sh.keptn.events.tests-finished`) -> clean up resources and print results

## Installation -  Deploy in your Kubernetes cluster

To deploy the current version of the *locust-service* in your Keptn Kubernetes cluster, clone the repo and apply the [`deploy/service.yaml`](deploy/service.yaml) file:

```console
kubectl apply -f deploy/service.yaml
```

This will install the `locust-service` into the `keptn` namespace, which you can verify using:

```console
kubectl -n keptn get deployment locust-service -o wide
kubectl -n keptn get pods -l run=locust-service
```

## Usage

The `locust-service` expects locust test files in the project specific Keptn repo. It expects those files to be available in the `locust` subfolder for a service in the stage you want to execute tests.

Here is an example on how to upload the `locust.py` test file via the Keptn CLI to the dev stage of project sockshop for the service carts:

```
keptn add-resource --project=sockshop --stage=dev --service=carts --resource=locust.py --resourceUri=locust/locust.py
```

### Defaults

If the users does not provide a specific configuration file (see next section) the `locust-service` will search for `locust/locust.py` in the specific Keptn repo and try to execute it. If the file does not exist the tests will be skipped.

### Providing a config file

A `locust.conf.yaml` file can be added for more complex configurations like multiple tests files for different stages. It is also possible to provide a [locust config file](https://docs.locust.io/en/stable/configuration.html#configuration-file) to further customize the tests. If no configuration file is specified the `locust-service` will proceed with the default setting. 

```
---
spec_version: '0.1.0'
workloads:
  - teststrategy: functional
    script: /locust/basic.py
    conf: /locust/locust.conf
  - teststrategy: performance_light
    script: /locust/basic.py
    conf: /locust/locust.conf
  - teststrategy: functional-light
    script: /locust/basic.py
```

The contents of `locust.conf.yaml` should be used as described in the following:
- If a "script" is given, it has to be used in the execution of the locust test.
- If "script" is not given, we can assume that the script is part of the "config" file that is referenced.
- If a "conf" is given, it should be used in the execution of the locust test.
- If "conf" is not given, the "script" will be executed with default setting.
- If both "script" and "conf" are missing, the integration skips the tests and indicate this in the result that is sent back to Keptn.

Examples for both the `locust.conf.yaml` and the [locust config file](https://docs.locust.io/en/stable/configuration.html#configuration-file) can be found in the [test-data/](test-data) directory.


## Uninstall -  Delete from your Kubernetes cluster

To delete the locust-service, delete using the [`deploy/service.yaml`](deploy/service.yaml) file:

```console
kubectl delete -f deploy/service.yaml
```

## Development

Development can be conducted using any Golang compatible IDE/editor (e.g., Jetbrains GoLand, VSCode with Go plugins).

It is recommended to make use of branches as follows:

* `master` contains the latest potentially unstable version
* `release-*` contains a stable version of the service (e.g., `release-0.1.0` contains version 0.1.0)
* create a new branch for any changes that you are working on, e.g., `feature/my-cool-stuff` or `bug/overflow`
* once ready, create a pull request from that branch back to the `master` branch

When writing code, it is recommended to follow the coding style suggested by the [Golang community](https://github.com/golang/go/wiki/CodeReviewComments).

### Where to start

If you don't care about the details, your first entrypoint is [eventhandlers.go](eventhandlers.go). Within this file you can add implementation for pre-defined Keptn Cloud events.
 
To better understand Keptn CloudEvents, please look at the [Keptn Spec](https://github.com/keptn/spec).
 
If you want to get more insights, please look into [main.go](main.go), [deploy/service.yaml](deploy/service.yaml), consult the [Keptn docs](https://keptn.sh/docs/) as well as existing [Keptn Core](https://github.com/keptn/keptn) and [Keptn Contrib](https://github.com/keptn-contrib/) services.

### Common tasks

* Run tests: `go test -race -v ./...`
* Deploy the service using `kubectl`: `kubectl apply -f deploy/`
* Delete/undeploy the service using `kubectl`: `kubectl delete -f deploy/`
* Watch the deployment using `kubectl`: `kubectl -n keptn get deployment locust-service -o wide`
* Get logs using `kubectl`: `kubectl -n keptn logs deployment/locust-service -f`
* Watch the deployed pods using `kubectl`: `kubectl -n keptn get pods -l run=locust-service`
* Deploy the service using [Skaffold](https://skaffold.dev/): `skaffold run --default-repo=your-docker-registry --tail` (Note: Replace `your-docker-registry` with your DockerHub username; also make sure to adapt the image name in [skaffold.yaml](skaffold.yaml))

### Testing Cloud Events

We have dummy cloud-events in the form of [RFC 2616](https://ietf.org/rfc/rfc2616.txt) requests in the [test-events/](test-events/) directory. These can be easily executed using third party plugins such as the [Huachao Mao REST Client in VS Code](https://marketplace.visualstudio.com/items?itemName=humao.rest-client).

## Automation

### GitHub Actions: Automated Pull Request Review

This repo uses [reviewdog](https://github.com/reviewdog/reviewdog) for automated reviews of Pull Requests. 

You can find the details in [.github/workflows/reviewdog.yml](.github/workflows/reviewdog.yml).

### GitHub Actions: Unit Tests

This repo has automated unit tests for pull requests. 

You can find the details in [.github/workflows/CI.yml](.github/workflows/CI.yml).


## License

Please find more information in the [LICENSE](LICENSE) file.
