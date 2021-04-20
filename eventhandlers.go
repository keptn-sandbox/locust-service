package main

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2" // make sure to use v2 cloudevents here
	keptn "github.com/keptn/go-utils/pkg/lib"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
)

/**
* Here are all the handler functions for the individual event
* See https://github.com/keptn/spec/blob/0.8.0-alpha/cloudevents.md for details on the payload
**/

const (
	LocustConfFilename       = "locust/locust.conf.yaml"
)

type LocustConf struct {
	SpecVersion string      `json:"spec_version" yaml:"spec_version"`
	Workloads   []*Workload `json:"workloads" yaml:"workloads"`
}

type Workload struct {
	TestStrategy      string            `json:"teststrategy" yaml:"teststrategy"`
	Script            string            `json:"script" yaml:"script"`
	Conf			string            `json:"conf" yaml:"conf"`
}

// Loads locust.conf for the current service
func getLocustConf(myKeptn *keptnv2.Keptn, project string, stage string, service string) (*LocustConf, error) {
	// if we run in a runlocal mode we are just getting the file from the local disk
	var fileContent []byte
	var err error

	log.Printf(fmt.Sprintf("Loading %s for %s.%s.%s", LocustConfFilename, project, stage, service))

	keptnResourceContent, err := myKeptn.GetKeptnResource(LocustConfFilename)

	if err != nil {
		logMessage := fmt.Sprintf("error when trying to load %s file for service %s on stage %s or project-level %s", LocustConfFilename, service, stage, project)
		log.Printf(logMessage)
		log.Println(err)
		return nil, errors.New(logMessage)
	}
	if keptnResourceContent == "" {
		// if no locust.conf file is available, this is not an error, as the service will proceed with the default workload
		log.Printf(fmt.Sprintf("no %s found", LocustConfFilename))
		return nil, nil
	}
	fileContent = []byte(keptnResourceContent)

	var locustConf *LocustConf
	locustConf, err = parseLocustConf(fileContent)
	if err != nil {
		logMessage := fmt.Sprintf("Couldn't parse %s file found for service %s in stage %s in project %s. Error: %s", LocustConfFilename, service, stage, project, err.Error())
		log.Fatal(logMessage)
		return nil, errors.New(logMessage)
	}

	log.Printf(fmt.Sprintf("Successfully loaded locust.conf.yaml with %d workloads", len(locustConf.Workloads)))

	return locustConf, nil
}

func getAllLocustResources(myKeptn *keptnv2.Keptn, project string, stage string, service string, tempDir string) error {
	resources, err := myKeptn.ResourceHandler.GetAllServiceResources(project, stage, service)

	if err != nil {
		log.Printf("Error getting locust files: %s", err.Error())
		return err
	}

	for _, resource := range resources {
		if strings.Contains(*resource.ResourceURI, "locust/") {
			fmt.Println("URI: "+*resource.ResourceURI)

			path := strings.Split(*resource.ResourceURI, "/")
			err = saveResourceToDirectory(tempDir,  path[len(path)-1], []byte(resource.ResourceContent))

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func saveResourceToDirectory(tempDir string, resourceName string, content []byte) error {
	targetFileName := fmt.Sprintf("%s/%s", tempDir, resourceName)

	resourceFile, err := os.Create(targetFileName)
	defer resourceFile.Close()

	_, err = resourceFile.Write(content)

	if err != nil {
		fmt.Printf("Failed to create tempfile: %s\n", err.Error())
		return err
	}

	return nil
}

// parses content and maps it to the LocustConf struct
func parseLocustConf(input []byte) (*LocustConf, error) {
	locustconf := &LocustConf{}
	err := yaml.Unmarshal([]byte(input), &locustconf)
	if err != nil {
		return nil, err
	}

	return locustconf, nil
}

// GenericLogKeptnCloudEventHandler is a generic handler for Keptn Cloud Events that logs the CloudEvent
func GenericLogKeptnCloudEventHandler(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data interface{}) error {
	log.Printf("Handling %s Event: %s", incomingEvent.Type(), incomingEvent.Context.GetID())
	log.Printf("CloudEvent %T: %v", data, data)

	return nil
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
		fmt.Printf("Failed to fetch file: %s\n", err.Error())
		return "", err
	}

	// Cut away folders from the path (if there are any)
	path := strings.Split(resourceName, "/")

	targetFileName := fmt.Sprintf("%s/%s", tempDir, path[len(path)-1])

	resourceFile, err := os.Create(targetFileName)
	defer resourceFile.Close()

	_, err = resourceFile.Write([]byte(requestedResourceContent))

	if err != nil {
		fmt.Printf("Failed to create tempfile: %s\n", err.Error())
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
			parts := strings.Split(lines[i], "/")
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

	// Send out a migrate.started CloudEvent
	// The get-sli.started cloud-event is new since Keptn 0.8.0 and is required to be send when the task is started
	_, err := myKeptn.SendTaskStartedEvent(&keptnv2.EventData{}, ServiceName)

	if err != nil {
		errMsg := fmt.Sprintf("Failed to send task started CloudEvent (%s), aborting...", err.Error())
		log.Println(errMsg)
		return err
	}

	serviceURL, err := getServiceURL(data)

	if err != nil {
		// report error
		log.Print(err)
		// send out a test.finished failed CloudEvent
		_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
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
	defer os.RemoveAll(tempDir)

	var locustconf *LocustConf
	locustconf, err = getLocustConf(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService())

	if err != nil {
		fmt.Printf("Failed to load Configuration file: %s", err.Error())
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
		locustFilename = "locustfile.py"
		fmt.Println("No locust.conf.yaml file provided. Continuing with default settings!")
	}

	fmt.Printf("TestStrategy=%s -> testFile=%s, serviceUrl=%s\n", data.Test.TestStrategy, locustFilename, serviceURL.String())

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
		} else {
			log.Println("Successfully fetched locust test file")
		}
	}

	// CAPTURE START TIME
	startTime := time.Now()

	// Download locust configuration file
	var locustConfiguration = ""
	if configFile != "" {
		locustConfiguration, err = getKeptnResource(myKeptn, configFile, tempDir)

		if err != nil {
			errMsg := fmt.Sprintf("Failed to fetch locust config file %s from config repo: %s", configFile, err.Error())
			log.Println(errMsg)
		} else {
			/*fetchErr := getAllLocustResources(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService(), tempDir)

			if fetchErr != nil {
				fmt.Println(fetchErr)
			}*/

			fmt.Println("Replacing locust configuration")
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

	fmt.Println(command)

	if locustResouceFilenameLocal == "" && locustConfiguration == "" {
		fmt.Println("Neither script nor conf is provided -> skipping tests")
	} else {
		log.Println("Running locust tests")
		str, err := keptn.ExecuteCommand("locust", command)

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
		errMsg := fmt.Sprintf("Failed to send task finished CloudEvent (%s), aborting...", err.Error())
		log.Println(errMsg)
		return err
	}

	return nil
}
