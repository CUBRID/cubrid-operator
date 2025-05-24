package certmanager

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// Certificate validity periods
	oneYear  = 365 * 24 * time.Hour
	oneMonth = 30 * 24 * time.Hour
	oneDay   = 24 * time.Hour
	oneHour  = time.Hour

	// DefaultCertificateDir is the default directory for storing certificates
	DefaultCertificateDir = "/tmp/k8s-webhook-server/serving-certs"
)

// CertManagerType represents the type of cert-manager to use
type CertManagerType string

const (
	// InternalCertManager represents the internal cert-manager
	InternalCertManager CertManagerType = "internal"
	// ExternalCertManager represents the external cert-manager
	ExternalCertManager CertManagerType = "external"
)

// Config holds the configuration for the cert-manager manager
type Config struct {
	// WebhookServiceName is the name of the webhook service
	WebhookServiceName string
	// WebhookNamespace is the namespace where the webhook server is deployed
	WebhookNamespace string
	// WebhookMutatingName is the name of the MutatingWebhookConfiguration
	WebhookMutatingName string

	// WebhookValidatingName is the name of the ValidatingWebhookConfiguration
	WebhookValidatingName string
	// WebhookCertDir is the directory where certificates are stored
	WebhookCertDir string
	// WebhookCertManagerType specifies whether to use internal or external cert-manager
	WebhookCertManagerType CertManagerType
	// WebhookSecretName is the name of the secret used by the webhook server
	WebhookSecretName string
	// CACertData is the PEM-encoded CA certificate data
	CACertData []byte
	// CertData is the PEM-encoded certificate data
	CertData []byte
	// KeyData is the PEM-encoded private key data
	KeyData []byte
	// WebhookDeploymentName is the name of the deployment to restart
	WebhookDeploymentName string
}

// Manager handles certificate management
type Manager struct {
	client client.Client
	config Config
}

// NewManager creates a new cert-manager manager
func NewManager(config *rest.Config, cfg Config) (*Manager, error) {
	// Create a new client
	c, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	// Set default values if not provided
	if cfg.WebhookServiceName == "" {
		return nil, fmt.Errorf("webhook name is required")
	}
	if cfg.WebhookNamespace == "" {
		return nil, fmt.Errorf("webhook namespace is required")
	}
	if cfg.WebhookCertDir == "" {
		cfg.WebhookCertDir = DefaultCertificateDir
	}
	if cfg.WebhookCertManagerType == "" {
		cfg.WebhookCertManagerType = InternalCertManager
	}
	if cfg.WebhookMutatingName == "" {
		cfg.WebhookMutatingName = "cubrid-operator-webhook-mutating"
	}
	if cfg.WebhookValidatingName == "" {
		cfg.WebhookValidatingName = "cubrid-operator-webhook-validating"
	}
	if cfg.WebhookDeploymentName == "" {
		cfg.WebhookDeploymentName = "cubrid-operator-controller-manager"
	}

	return &Manager{
		client: c,
		config: cfg,
	}, nil
}

// GetCertDir returns the certificate directory
func (m *Manager) GetCertDir() string {
	return m.config.WebhookCertDir
}

// isPodReady checks if the pod is in ready state and is newly created
func (m *Manager) isPodReady() (bool, error) {
	logger := log.Log.WithName("cert-manager")

	// Get pods owned by this deployment
	podList := &corev1.PodList{}
	err := m.client.List(context.Background(), podList, client.InNamespace(m.config.WebhookNamespace),
		client.MatchingLabels(map[string]string{"app": m.config.WebhookDeploymentName}))
	if err != nil {
		return false, fmt.Errorf("failed to list pods: %v", err)
	}

	// If there's only one pod, no need to check readiness
	if len(podList.Items) <= 1 {
		logger.Info("Only one or no pods found, skipping readiness check")
		return true, nil
	}

	// Find the newest pod
	var newestPod *corev1.Pod
	for _, pod := range podList.Items {
		if newestPod == nil || pod.CreationTimestamp.After(newestPod.CreationTimestamp.Time) {
			newestPod = &pod
		}
	}

	if newestPod == nil {
		return false, nil
	}

	// Check if the newest pod is ready
	for _, condition := range newestPod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			logger.Info("Newest pod is ready", "pod", newestPod.Name)
			return true, nil
		}
	}

	return false, nil
}

// watchPodReady watches the pod readiness and updates CA Bundle when pod is ready
func (m *Manager) watchPodReady() {
	logger := log.Log.WithName("cert-manager")
	logger.Info("Starting to watch pod readiness")

	for {
		ready, err := m.isPodReady()
		if err != nil {
			logger.Error(err, "Failed to check pod readiness")
			time.Sleep(time.Second)
			continue
		}

		if ready {
			// Update caBundle in WebhookConfiguration
			if err := m.updateWebhookConfiguration(); err != nil {
				logger.Error(err, "Unable to update webhook configuration")
				time.Sleep(time.Second)
				continue
			}
			logger.Info("CA Bundle updated successfully")
			return
		}

		time.Sleep(time.Second)
	}
}

// Start starts the cert-manager manager
func (m *Manager) Start() error {
	logger := log.Log.WithName("cert-manager")
	// Return error if external cert-manager is selected
	if m.config.WebhookCertManagerType == ExternalCertManager {
		return fmt.Errorf("internal cert-manager should not be started when external cert-manager is selected")
	}

	//  Check existing certificates
	secret := &corev1.Secret{}
	err := m.client.Get(context.Background(), client.ObjectKey{
		Namespace: m.config.WebhookNamespace,
		Name:      m.config.WebhookSecretName,
	}, secret)

	if err == nil && secret != nil {
		// If Secret exists, validate certificates
		if certData, ok := secret.Data["tls.crt"]; ok && len(certData) > 0 {
			if keyData, ok := secret.Data["tls.key"]; ok && len(keyData) > 0 {
				if caData, ok := secret.Data["ca.crt"]; ok && len(caData) > 0 {
					// Validate certificates
					if m.isValidCertificate(certData, keyData, caData) {
						logger.Info("Valid existing certificates found in secret, skipping certificate creation")

						m.config.CACertData = caData
						m.config.CertData = certData
						m.config.KeyData = keyData

						// Start watching pod readiness in a separate goroutine
						go m.watchPodReady()

						// Start certificate monitoring
						go m.watchCertificate()
						return nil
					}
				}
			}
		}
	}

	// If certificates do not exist or are invalid, create new ones
	logger.Info("Creating new certificates")
	if err := m.createSelfSignedCertificate(); err != nil {
		logger.Error(err, "Unable to create self-signed certificate")
		return err
	}

	// Save the generated certificates to Secret
	if err := m.saveCertificatesToSecret(); err != nil {
		logger.Error(err, "Unable to save certificates to secret")
		return err
	}

	// Start watching pod readiness in a separate goroutine
	go m.watchPodReady()

	// Start certificate monitoring
	go m.watchCertificate()

	return nil
}

// isValidCertificate checks if the certificate is valid and not expired
func (m *Manager) isValidCertificate(certData, keyData, caData []byte) bool {
	logger := log.Log.WithName("cert-manager")
	// PEM 디코딩
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		logger.Error(nil, "Failed to decode PEM certificate")
		return false
	}

	caBlock, _ := pem.Decode(caData)
	if caBlock == nil {
		logger.Error(nil, "Failed to decode PEM CA certificate")
		return false
	}

	// 서버 인증서 파싱
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		logger.Error(err, "Unable to parse server certificate")
		return false
	}

	// CA 인증서 파싱
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		logger.Error(err, "Unable to parse CA certificate")
		return false
	}

	// 현재 시간이 인증서의 유효 기간 내에 있는지 확인
	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		logger.Error(nil, "Certificate is not valid")
		return false
	}
	if now.Before(caCert.NotBefore) || now.After(caCert.NotAfter) {
		logger.Error(nil, "CA certificate is not valid")
		return false
	}

	// 키와 인증서가 서로 매칭되는지 확인
	_, err = tls.X509KeyPair(certData, keyData)
	if err != nil {
		logger.Error(err, "Key and certificate do not match")
		return false
	}

	// 서버 인증서가 CA 인증서로 서명되었는지 확인
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now,
	})
	if err != nil {
		logger.Error(err, "Certificate is not verified")
		return false
	}

	logger.Info("Certificate is valid")
	return true
}

// createSelfSignedCertificate creates a self-signed certificate
func (m *Manager) createSelfSignedCertificate() error {
	logger := log.Log.WithName("cert-manager")
	// Check if certificate directory exists
	if _, err := os.Stat(m.config.WebhookCertDir); os.IsNotExist(err) {
		logger.Info("certificate directory does not exist", "certDir", m.config.WebhookCertDir)
	}

	// Create certificate directory if it doesn't exist
	logger.Info("creating certificate directory", "certDir", m.config.WebhookCertDir)
	if err := os.MkdirAll(m.config.WebhookCertDir, 0755); err != nil {
		return err
	}

	// Generate CA private key
	logger.Info("generating CA private key")
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Create CA certificate template
	logger.Info("creating CA certificate template")
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"cubrid"},
			CommonName:   "cubrid-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(oneYear),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Create CA certificate
	logger.Info("creating CA certificate")
	caDerBytes, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return err
	}

	// Save CA certificate
	logger.Info("saving CA certificate")
	caCertBuffer := new(bytes.Buffer)
	if err := pem.Encode(caCertBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: caDerBytes}); err != nil {
		logger.Error(err, "Unable to encode CA certificate")
		return err
	}

	// Store the PEM-encoded CA certificate in memory for WebhookConfiguration update
	m.config.CACertData = caCertBuffer.Bytes()

	// Generate server private key
	logger.Info("generating server private key")
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Create server certificate template
	logger.Info("creating server certificate template")
	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"cubrid"},
			CommonName:   m.config.WebhookServiceName,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(oneYear),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{
			m.config.WebhookServiceName + "." + m.config.WebhookNamespace + ".svc",
			m.config.WebhookServiceName + "." + m.config.WebhookNamespace + ".svc.cluster.local",
		},
	}

	// Create server certificate signed by CA
	logger.Info("creating server certificate")
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &caTemplate, &serverPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return err
	}

	// Save server certificate
	logger.Info("saving server certificate")
	certBuffer := new(bytes.Buffer)
	if err := pem.Encode(certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		logger.Error(err, "Unable to encode server certificate")
		return err
	}
	m.config.CertData = certBuffer.Bytes()

	// Save server private key
	logger.Info("saving server private key")
	keyBuffer := new(bytes.Buffer)
	if err := pem.Encode(keyBuffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverPrivateKey)}); err != nil {
		logger.Error(err, "Unable to encode server private key")
		return err
	}
	m.config.KeyData = keyBuffer.Bytes()

	logger.Info("certificates created successfully")
	return nil
}

// updateWebhookConfiguration updates the caBundle in WebhookConfiguration
func (m *Manager) updateWebhookConfiguration() error {
	logger := log.Log.WithName("cert-manager")

	// CA 인증서 데이터 확인
	if len(m.config.CACertData) == 0 {
		return fmt.Errorf("CA certificate data is empty")
	}

	// Update MutatingWebhookConfiguration
	mutatingWebhook := &admissionv1.MutatingWebhookConfiguration{}
	if err := m.client.Get(context.Background(), client.ObjectKey{Name: m.config.WebhookMutatingName}, mutatingWebhook); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get MutatingWebhookConfiguration: %v", err)
		}
		logger.Info("MutatingWebhookConfiguration not found", "name", m.config.WebhookMutatingName)
	} else {
		for i := range mutatingWebhook.Webhooks {
			mutatingWebhook.Webhooks[i].ClientConfig.CABundle = m.config.CACertData
		}
		if err := m.client.Update(context.Background(), mutatingWebhook); err != nil {
			return fmt.Errorf("failed to update MutatingWebhookConfiguration: %v", err)
		}
		logger.Info("Updated MutatingWebhookConfiguration", "name", m.config.WebhookMutatingName)
	}

	// Update ValidatingWebhookConfiguration
	validatingWebhook := &admissionv1.ValidatingWebhookConfiguration{}
	if err := m.client.Get(context.Background(), client.ObjectKey{Name: m.config.WebhookValidatingName}, validatingWebhook); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get ValidatingWebhookConfiguration: %v", err)
		}
		logger.Info("ValidatingWebhookConfiguration not found", "name", m.config.WebhookValidatingName)
	} else {
		for i := range validatingWebhook.Webhooks {
			validatingWebhook.Webhooks[i].ClientConfig.CABundle = m.config.CACertData
		}
		if err := m.client.Update(context.Background(), validatingWebhook); err != nil {
			return fmt.Errorf("failed to update ValidatingWebhookConfiguration: %v", err)
		}
		logger.Info("Updated ValidatingWebhookConfiguration", "name", m.config.WebhookValidatingName)
	}

	return nil
}

// certificatesExistAndValid checks if certificates are valid based on expiration
func (m *Manager) certificatesExistAndValid() (bool, error) {
	logger := log.Log.WithName("cert-manager")

	// Get certificate from secret
	secret := &corev1.Secret{}
	err := m.client.Get(context.Background(), client.ObjectKey{
		Namespace: m.config.WebhookNamespace,
		Name:      m.config.WebhookSecretName,
	}, secret)
	if err != nil {
		logger.Error(err, "Failed to get secret")
		return false, err
	}

	certData, ok := secret.Data["tls.crt"]
	if !ok {
		logger.Error(nil, "Certificate data not found in secret")
		return false, nil
	}

	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		logger.Error(nil, "Failed to decode PEM certificate")
		return false, nil
	}

	x509Cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		logger.Error(err, "Failed to parse certificate")
		return false, err
	}

	// Check if certificate is valid and not near expiration
	now := time.Now()
	if now.After(x509Cert.NotAfter) {
		logger.Info("Certificate has expired", "notAfter", x509Cert.NotAfter)
		return false, nil
	}

	if time.Until(x509Cert.NotAfter) < oneMonth {
		logger.Info("Certificate is near expiration", "notAfter", x509Cert.NotAfter, "timeUntil", time.Until(x509Cert.NotAfter))
		return false, nil
	}

	logger.Info("Certificate is valid", "notAfter", x509Cert.NotAfter)
	return true, nil
}

// saveCertificatesToSecret saves the generated certificates to a Kubernetes Secret
func (m *Manager) saveCertificatesToSecret() error {
	logger := log.Log.WithName("cert-manager")

	if m.config.WebhookSecretName == "" {
		return fmt.Errorf("webhook secret name is required")
	}

	// Create or update Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.config.WebhookSecretName,
			Namespace: m.config.WebhookNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": m.config.CertData,
			"tls.key": m.config.KeyData,
			"ca.crt":  m.config.CACertData,
		},
	}

	// Create or update Secret
	if err := m.client.Create(context.Background(), secret); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		logger.Info("Secret already exists, updating...")
		if err := m.client.Update(context.Background(), secret); err != nil {
			return err
		}
	}

	logger.Info("Secret created/updated successfully")
	return nil
}

// watchCertificate watches the certificate and renews it when needed
func (m *Manager) watchCertificate() {
	logger := log.Log.WithName("cert-manager")
	logger.Info("Starting to watch certificate")

	for {
		// Check if certificate needs renewal
		exist, err := m.certificatesExistAndValid()
		if err != nil {
			if errors.IsNotFound(err) {
				// Secret not found, create new certificates
				logger.Info("Secret not found, creating new certificates")
				if err := m.createSelfSignedCertificate(); err != nil {
					logger.Error(err, "Unable to create new certificate")
					time.Sleep(oneHour)
					continue
				}

				// Save to secret
				if err := m.saveCertificatesToSecret(); err != nil {
					logger.Error(err, "Unable to save certificates to secret")
					time.Sleep(oneHour)
					continue
				}

				// Note: This will be done after pod restart in the new pod
				logger.Info("New certificates saved to secret, pod will be restarted to apply changes")

				// Trigger pod restart
				if err := m.restartPod(); err != nil {
					logger.Error(err, "Unable to restart pod")
					time.Sleep(oneHour)
					continue
				}

				logger.Info("Triggered rolling update")
				time.Sleep(oneHour)
				continue
			} else {
				// Other errors, just log and continue
				logger.Error(err, "Unable to check certificate validity")
				time.Sleep(oneHour)
				continue
			}
		}

		if !exist {
			// Certificate exists but needs renewal
			if err := m.createSelfSignedCertificate(); err != nil {
				logger.Error(err, "Unable to create new certificate")
				time.Sleep(oneHour)
				continue
			}

			// Save to secret
			if err := m.saveCertificatesToSecret(); err != nil {
				logger.Error(err, "Unable to save certificates to secret")
				time.Sleep(oneHour)
				continue
			}

			// Note: This will be done after pod restart in the new pod
			logger.Info("New certificates saved to secret, pod will be restarted to apply changes")

			// Trigger pod restart
			if err := m.restartPod(); err != nil {
				logger.Error(err, "Unable to restart pod")
				time.Sleep(oneHour)
				continue
			}

			logger.Info("Triggered rolling update")
			time.Sleep(oneHour)
			continue
		}

		time.Sleep(oneHour)
	}
}

// restartPod restarts the pod by updating the deployment's annotation
func (m *Manager) restartPod() error {
	logger := log.Log.WithName("cert-manager")

	// Get the deployment
	deployment := &appsv1.Deployment{}
	err := m.client.Get(context.Background(), client.ObjectKey{
		Namespace: m.config.WebhookNamespace,
		Name:      m.config.WebhookDeploymentName,
	}, deployment)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	// Update the deployment's annotation to trigger a restart
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	// Update the deployment
	if err := m.client.Update(context.Background(), deployment); err != nil {
		return fmt.Errorf("failed to update deployment: %v", err)
	}

	logger.Info("Successfully triggered pod restart")
	return nil
}
