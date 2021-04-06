package main

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"net/url"
	"os"
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

	targetFileName := fmt.Sprintf("%s/%s", tempDir, resourceName)

	resourceFile, err := os.Create(targetFileName)
	defer resourceFile.Close()

	_, err = resourceFile.Write([]byte(requestedResourceContent))

	if err != nil {
		fmt.Printf("Failed to create tempfile: %s\n", err.Error())
		return "", err
	}

	return targetFileName, nil
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
	var numUsers int
	var timeSpend string

	// create a tempdir
	tempDir, err := ioutil.TempDir("", "locust")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// ToDo: this should be configured similar to jmeter.conf
	// https://github.com/keptn/keptn/tree/master/jmeter-service#custom-workloads
	var locustconf *LocustConf
	locustconf, err = getLocustConf(myKeptn, myKeptn.Event.GetProject(), myKeptn.Event.GetStage(), myKeptn.Event.GetService())

	fmt.Println("Locust configuration loaded!")
	fmt.Println(locustconf)

	for _, workload := range locustconf.Workloads {
		if workload.TestStrategy == data.Test.TestStrategy {
			locustFilename = workload.Script

			// TODO: Implement locust config file
		}
	}

	if data.Test.TestStrategy == "performance" {
		locustFilename = "locust/load.py"
		numUsers = 100
		timeSpend = "2m"
	} else if data.Test.TestStrategy == "functional" {
		locustFilename = "locust/basic.py"
		numUsers = 1
		timeSpend = "1m"
	} else {
		locustFilename = "locust/health.py"
		numUsers = 1
		timeSpend = "30s"
	}

	fmt.Printf("TestStrategy=%s -> numUsers=%d, testFile=%s, serviceUrl=%s\n", data.Test.TestStrategy, numUsers, locustFilename, serviceURL.String())

	locustResouceFilenameLocal, err := getKeptnResource(myKeptn, locustFilename, tempDir)

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

	// CAPTURE START TIME
	startTime := time.Now()

	// locust -f locust/test.py --headless --host=HOST --users=1 --run-time=1m
	str, err := keptn.ExecuteCommand("locust", []string{
		"-f", locustResouceFilenameLocal,
		"--headless", "--only-summary",
		"--host=" + serviceURL.String(),
		fmt.Sprintf("--users=%d", numUsers),
		fmt.Sprintf("--run-time=%s", timeSpend)})

	log.Print(str)

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
