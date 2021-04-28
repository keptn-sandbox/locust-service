package environment

import (
	"context"
	"fmt"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const defaultNamespace = "keptn"
const secretPrefix = "locust"

type EnvironmentProvider struct {
	KubeAPI                v1.CoreV1Interface
	KeptnNamespaceProvider StringSupplier
}

func NewEnvironmentProvider(kubeAPI v1.CoreV1Interface) *EnvironmentProvider {
	return &EnvironmentProvider{
		KubeAPI:                kubeAPI,
		KeptnNamespaceProvider: envBasedStringSupplier("POD_NAMESPACE", defaultNamespace),
	}
}

// PrepareEnvironment creates a list of environment variables by extracting them from a secret based on project, stage and service
func (e EnvironmentProvider) PrepareEnvironment(project string, stage string, service string) []string {
	environment := []string{}
	secretName := fmt.Sprintf("%s-%s-%s-%s", secretPrefix, project, stage, service)
	log.Printf("Prepare data of secret %s as environment", secretName)

	secret, err := e.KubeAPI.Secrets(e.KeptnNamespaceProvider()).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Unable to get secret %s: %s", secretName, err)
		return environment
	}

	for key, value := range secret.Data {
		environment = append(environment, fmt.Sprintf("%s=%s", key, string(value)))
	}
	return environment
}

type StringSupplier func() string

func envBasedStringSupplier(envVarName, defaultVal string) StringSupplier {
	return func() string {
		ns := os.Getenv(envVarName)
		if ns != "" {
			return ns
		}
		return defaultVal
	}
}
