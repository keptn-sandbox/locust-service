package environment

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func createK8sSecretObj(name string, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
		Type: "Opaque",
	}
}

func assertEnvironmentVariable(t *testing.T, environment []string, key string, value string) {
	cmd := exec.Command("sh", "-c", "echo -n $"+key)
	cmd.Env = append(cmd.Env, environment...)
	out, _ := cmd.CombinedOutput()
	assert.Equal(t, value, string(out))
}

func TestPrepareEnvironment(t *testing.T) {
	key1, value1 := "key1", "value1"
	key2, value2 := "key2", "value2"

	kubernetes := k8sfake.NewSimpleClientset()
	environmentProvider := NewEnvironmentProvider(kubernetes.CoreV1())
	namespace := environmentProvider.KeptnNamespaceProvider()

	secretData := map[string][]byte{key1: []byte(value1), key2: []byte(value2)}
	k8sSecret := createK8sSecretObj("locust-project-stage-service", namespace, secretData)
	kubernetes.CoreV1().Secrets(namespace).Create(context.TODO(), k8sSecret, metav1.CreateOptions{})

	environment := environmentProvider.PrepareEnvironment("project", "stage", "service")
	assert.NotNil(t, environment)
	assertEnvironmentVariable(t, environment, key1, value1)
	assertEnvironmentVariable(t, environment, key2, value2)
}

func TestPrepareEnvironment_SecretNotFound(t *testing.T) {
	kubernetes := k8sfake.NewSimpleClientset()
	environmentProvider := NewEnvironmentProvider(kubernetes.CoreV1())
	namespace := environmentProvider.KeptnNamespaceProvider()

	k8sSecret := createK8sSecretObj("locust-project-stage-unknown", namespace, map[string][]byte{})
	kubernetes.CoreV1().Secrets(namespace).Create(context.TODO(), k8sSecret, metav1.CreateOptions{})

	environment := environmentProvider.PrepareEnvironment("project", "stage", "service")
	assert.NotNil(t, environment)
	assert.Empty(t, environment)
}

func TestPrepareEnvironment_ComplexValue(t *testing.T) {
	key1, value1 := "key1", "complex value=1"
	key2, value2 := "key2", "\"value2\""

	kubernetes := k8sfake.NewSimpleClientset()
	environmentProvider := NewEnvironmentProvider(kubernetes.CoreV1())
	namespace := environmentProvider.KeptnNamespaceProvider()

	secretData := map[string][]byte{key1: []byte(value1), key2: []byte(value2)}
	k8sSecret := createK8sSecretObj("locust-project-stage-service", namespace, secretData)
	kubernetes.CoreV1().Secrets(namespace).Create(context.TODO(), k8sSecret, metav1.CreateOptions{})

	environment := environmentProvider.PrepareEnvironment("project", "stage", "service")
	assert.NotNil(t, environment)
	assertEnvironmentVariable(t, environment, key1, value1)
	assertEnvironmentVariable(t, environment, key2, value2)
}
