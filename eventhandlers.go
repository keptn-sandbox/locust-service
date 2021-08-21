package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	cloudevents "github.com/cloudevents/sdk-go/v2" // make sure to use v2 cloudevents here
	env "github.com/keptn-sandbox/locust-service/pkg/environment"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	k8sutils "github.com/keptn/kubernetes-utils/pkg"
)

/**
* Here are all the handler functions for the individual event
* See https://github.com/keptn/spec/blob/0.8.0-alpha/cloudevents.md for details on the payload
**/

// Locust configuration file path
const (
	// LocustConfFilename defines the path to the locust.conf.yaml
	LocustConfFilename = "locust/locust.conf.yaml"
	// DefaultLocustFilename defines the path to the default locustfile.py
	DefaultLocustFilename = "locust/locustfile.py"
)

// LocustConf Configuration file type
type LocustConf struct {
	SpecVersion string      `json:"spec_version" yaml:"spec_version"`
	Workloads   []*Workload `json:"workloads" yaml:"workloads"`
}

// Workload of Keptn stage
type Workload struct {
	TestStrategy string `json:"teststrategy" yaml:"teststrategy"`
	Script       string `json:"script" yaml:"script"`
	Conf         string `json:"conf" yaml:"conf"`
}

// Loads locust.conf for the current service
func getLocustConf(myKeptn *keptnv2.Keptn, project string, stage string, service string) (*LocustConf, error) {
	var err error

	log.Printf("Loading %s for %s.%s.%s", LocustConfFilename, project, stage, service)

	keptnResourceContent, err := myKeptn.GetKeptnResource(LocustConfFilename)

	if err != nil {
		logMessage := fmt.Sprintf("error when trying to load %s file for service %s on stage %s or project-level %s: %s", LocustConfFilename, service, stage, project, err.Error())
		return nil, errors.New(logMessage)
	}
	if len(keptnResourceContent) == 0 {
		// if no locust.conf file is available, this is not an error, as the service will proceed with the default workload
		log.Printf("no %s found", LocustConfFilename)
		return nil, nil
	}

	var locustConf *LocustConf
	locustConf, err = parseLocustConf(keptnResourceContent)
	if err != nil {
		logMessage := fmt.Sprintf("Couldn't parse %s file found for service %s in stage %s in project %s. Error: %s", LocustConfFilename, service, stage, project, err.Error())
		return nil, errors.New(logMessage)
	}

	log.Printf("Successfully loaded locust.conf.yaml with %d workloads", len(locustConf.Workloads))

	return locustConf, nil
}

func getAllLocustResources(myKeptn *keptnv2.Keptn, project string, stage string, service string, tempDir string) error {
	resources, err := myKeptn.ResourceHandler.GetAllServiceResources(project, stage, service)

	if err != nil {
		log.Printf("Error getting locust files: %s", err.Error())
		return err
	}

	for _, resource := range resources {
		if strings.Contains(*resource.ResourceURI, "locust/") && !strings.HasSuffix(*resource.ResourceURI, ".conf") {
			_, err := getKeptnResource(myKeptn, *resource.ResourceURI, tempDir)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

// parses content and maps it to the LocustConf struct
func parseLocustConf(input []byte) (*LocustConf, error) {
	locustconf := &LocustConf{}
	err := yaml.Unmarshal(input, &locustconf)
	if err != nil {
		return nil, err
	}

	return locustconf, nil
}

// GenericLogKeptnCloudEventHandler is a generic handler for Keptn Cloud Events that logs the CloudEvent
func GenericLogKeptnCloudEventHandler(incomingEvent cloudevents.Event, data interface{}) {
	log.Printf("Handling %s Event: %s", incomingEvent.Type(), incomingEvent.Context.GetID())
	log.Printf("CloudEvent %T: %v", data, data)
}

// getServiceURL extracts the deployment URI from the test.triggered cloud-event
func getServiceURL(data *keptnv2.TestTriggeredEventData) (*url.URL, error) {
	if len(data.Deployment.DeploymentURIsPublic) > 0 && data.Deployment.DeploymentURIsPublic[0] != "" {
		return url.Parse(data.Deployment.DeploymentURIsPublic[0])
	} else if len(data.Deployment.DeploymentURIsLocal) > 0 && data.Deployment.DeploymentURIsLocal[0] != "" {
		return url.Parse(data.Deployment.DeploymentURIsLocal[0])
	}

	return nil, errors.New("no deployment URI included in event")
}

// getKeptnResource fetches a resource from Keptn config repo and stores it in a temp directory
func getKeptnResource(myKeptn *keptnv2.Keptn, resourceName string, tempDir string) (string, error) {
	requestedResourceContent, err := myKeptn.GetKeptnResource(resourceName)

	if err != nil {
		log.Printf("Failed to fetch file: %s\n", err.Error())
		return "", err
	}

	// Cut away folders from the path (if there are any)
	path := strings.Split(resourceName, "/")

	targetFileName := fmt.Sprintf("%s/%s", tempDir, path[len(path)-1])

	resourceFile, err := os.Create(targetFileName)
	defer resourceFile.Close()

	_, err = resourceFile.Write(requestedResourceContent)

	if err != nil {
		log.Printf("Failed to create tempfile: %s\n", err.Error())
		return "", err
	}

	return targetFileName, nil
}

func replaceLocustFileName(filename string, tempDir string) {
	input, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln(err)
	}

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, "locustfile") {
			locustURI := strings.Split(lines[i], "=")
			parts := strings.Split(locustURI[len(locustURI)-1], "/")
			lines[i] = fmt.Sprintf("locustfile = %s/%s", tempDir, parts[len(parts)-1])
		}
	}

	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(filename, []byte(output), 0644)
	if err != nil {
		log.Fatalln(err)
	}
}

// HandleTestTriggeredEvent handles test.triggered events by calling locust
func HandleTestTriggeredEvent(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *keptnv2.TestTriggeredEventData) error {
	log.Printf("Handling test.triggered Event: %s", incomingEvent.Context.GetID())

	// CAPTURE START TIME
	startTime := time.Now()

	// Send out a migrate.started CloudEvent
	// The get-sli.started cloud-event is new since Keptn 0.8.0 and is required to be send when the task is started
	_, err := myKeptn.SendTaskStartedEvent(&keptnv2.EventData{}, ServiceName)

	if err != nil {
		log.Printf("Failed to send task started CloudEvent (%s), aborting... \n", err.Error())
		return err
	}

	serviceURL, err := getServiceURL(data)

	if err != nil {
		// report error
		log.Print(err)
		// send out a test.finished failed CloudEvent
		myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
			Status:  keptnv2.StatusErrored,
			Result:  keptnv2.ResultFailed,
			Message: err.Error(),
		}, ServiceName)
	}

	var locustFilename string

	// create a tempdir
	tempDir, err := ioutil.TempDir("", "locust")
	if err != nil {
		log.Fatal(err)
	}
	//defer os.RemoveAll(tempDir)

	var locustconf *LocustConf
	locustconf, err = getLocustConf(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService())

	if err != nil {
		log.Printf("Failed to load Configuration file: %s", err.Error())
	}

	var configFile = ""

	if locustconf != nil {
		for _, workload := range locustconf.Workloads {
			if workload.TestStrategy == data.Test.TestStrategy {
				if workload.Script != "" {
					locustFilename = workload.Script
				} else {
					locustFilename = ""
				}

				if workload.Conf != "" {
					configFile = workload.Conf
				}
			}
		}
	} else {
		locustFilename = DefaultLocustFilename
		_, err = getKeptnResource(myKeptn, locustFilename, tempDir)
		if err != nil {
			log.Println("No locust.conf.yaml file provided. Default locust file also doesn't exist. Skipping locust tests!")

			endTime := time.Now()

			finishedEvent := &keptnv2.TestFinishedEventData{
				Test: keptnv2.TestFinishedDetails{
					Start: startTime.Format(time.RFC3339),
					End:   endTime.Format(time.RFC3339),
				},
				EventData: keptnv2.EventData{
					Result:  keptnv2.ResultPass,
					Status:  keptnv2.StatusSucceeded,
					Message: "No locust.conf.yaml file provided. Default locust file also doesn't exist. Skipping locust tests!",
				},
			}

			myKeptn.SendTaskFinishedEvent(finishedEvent, ServiceName)

			return nil
		}
		log.Println("No locust.conf.yaml file provided. Continuing with default settings!")
	}

	msg := fmt.Sprintf("TestStrategy=%s -> testFile=%s, serviceUrl=%s\n", data.Test.TestStrategy, locustFilename, serviceURL.String())
	log.Println(msg)

	_, err = myKeptn.SendTaskStatusChangedEvent(&keptnv2.EventData{
		Message: msg,
	}, ServiceName)

	if err != nil {
		log.Printf("Could not send status changed event: %s", err.Error())
	}

	var locustResouceFilenameLocal = ""
	if locustFilename != "" {
		locustResouceFilenameLocal, err = getKeptnResource(myKeptn, locustFilename, tempDir)

		// FYI you do not need to "fail" if sli.yaml is missing, you can also assume smart defaults like we do
		// in keptn-contrib/dynatrace-service and keptn-contrib/prometheus-service
		if err != nil {
			// failed to fetch sli config file
			errMsg := fmt.Sprintf("Failed to fetch locust file %s from config repo: %s", locustFilename, err.Error())
			log.Println(errMsg)
			// send a get-sli.finished event with status=error and result=failed back to Keptn

			_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
				Status:  keptnv2.StatusErrored,
				Result:  keptnv2.ResultFailed,
				Message: errMsg,
			}, ServiceName)

			return err
		}

		log.Println("Successfully fetched locust test file")
	}

	// Download locust configuration file
	var locustConfiguration = ""
	if configFile != "" {
		locustConfiguration, err = getKeptnResource(myKeptn, configFile, tempDir)

		if err != nil {
			log.Printf("Failed to fetch locust config file %s from config repo: %s \n", configFile, err.Error())
		} else {
			fetchErr := getAllLocustResources(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), tempDir)

			if fetchErr != nil {
				log.Println(fetchErr)
			}

			log.Println("Replacing locust configuration")
			replaceLocustFileName(locustConfiguration, tempDir)
		}
	}

	command := []string{
		"--headless", "--only-summary",
		"--host=" + serviceURL.String(),
	}

	if locustResouceFilenameLocal != "" {
		command = append(command, fmt.Sprintf("-f=%s", locustResouceFilenameLocal))
	}

	if locustConfiguration != "" {
		command = append(command, fmt.Sprintf("--config=%s", locustConfiguration))
	} else {
		// Set default values
		command = append(command, fmt.Sprintf("--users=%d", 10))
		command = append(command, fmt.Sprintf("--run-time=%s", "2m"))
	}

	if locustResouceFilenameLocal == "" && locustConfiguration == "" {
		log.Println("Neither script nor conf is provided -> skipping tests")
	} else {
		log.Println("Prepare environment")
		kubeClient, _ := k8sutils.GetKubeAPI(true)
		EnvironmentProvider := env.NewEnvironmentProvider(kubeClient)
		environment := EnvironmentProvider.PrepareEnvironment(myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService())
		log.Println("Running locust tests")
		str, err := ExecuteCommandWithEnv("locust", command, environment)

		log.Println("Finished running locust tests")
		log.Println(str)

		if err != nil {
			// report error
			log.Print(err)
			// send out a test.finished failed CloudEvent
			_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
				Status:  keptnv2.StatusErrored,
				Result:  keptnv2.ResultFailed,
				Message: err.Error(),
			}, ServiceName)

			return err
		}
	}

	endTime := time.Now()

	// Done

	finishedEvent := &keptnv2.TestFinishedEventData{
		Test: keptnv2.TestFinishedDetails{
			Start: startTime.Format(time.RFC3339),
			End:   endTime.Format(time.RFC3339),
		},
		EventData: keptnv2.EventData{
			Result:  keptnv2.ResultPass,
			Status:  keptnv2.StatusSucceeded,
			Message: "Locust test finished successfully",
		},
	}

	// Finally: send out a test.finished CloudEvent
	_, err = myKeptn.SendTaskFinishedEvent(finishedEvent, ServiceName)

	if err != nil {
		log.Printf("Failed to send task finished CloudEvent (%s), aborting...\n", err.Error())
		return err
	}

	return nil
}

// borrowed from go-utils, remove when https://github.com/keptn/go-utils/pull/286 is merged and new go-utils version available
func ExecuteCommandWithEnv(command string, args []string, env []string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error executing command %s %s: %s\n%s", command, strings.Join(args, " "), err.Error(), string(out))
	}
	return string(out), nil
}
