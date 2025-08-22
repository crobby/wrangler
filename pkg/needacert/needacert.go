package needacert

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
	"time"

	"github.com/moby/locker"
	admissionregcontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/admissionregistration.k8s.io/v1"
	apiextcontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/gvk"
	"github.com/rancher/wrangler/v3/pkg/slice"
	adminregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/cert"
)

var (
	SecretAnnotation = "need-a-cert.cattle.io/secret-name"
	DNSAnnotation    = "need-a-cert.cattle.io/dns-name"
)

const (
	byServiceIndex = "byService"
)

func Register(ctx context.Context,
	secrets corecontrollers.SecretController,
	service corecontrollers.ServiceController,
	mutatingController admissionregcontrollers.MutatingWebhookConfigurationController,
	validatingController admissionregcontrollers.ValidatingWebhookConfigurationController,
	crdController apiextcontrollers.CustomResourceDefinitionController) {
	h := handler{
		secretsCache:       secrets.Cache(),
		secrets:            secrets,
		serviceCache:       service.Cache(),
		mutatingWebHooks:   mutatingController,
		validatingWebHooks: validatingController,
		crds:               crdController,
	}

	mutatingController.Cache().AddIndexer(byServiceIndex, mutatingWebhookServices)
	validatingController.Cache().AddIndexer(byServiceIndex, validatingWebhookServices)
	crdController.Cache().AddIndexer(byServiceIndex, crdWebhookServices)

	mutatingController.OnChange(ctx, "need-a-cert", h.OnMutationWebhookChange)
	validatingController.OnChange(ctx, "need-a-cert", h.OnValidatingWebhookChange)
	crdController.OnChange(ctx, "need-a-cert", h.OnCRDChange)
	service.OnChange(ctx, "need-a-cert", h.OnService)
}

type handler struct {
	locker             locker.Locker
	secretsCache       corecontrollers.SecretCache
	secrets            corecontrollers.SecretClient
	serviceCache       corecontrollers.ServiceCache
	mutatingWebHooks   admissionregcontrollers.MutatingWebhookConfigurationController
	validatingWebHooks admissionregcontrollers.ValidatingWebhookConfigurationController
	crds               apiextcontrollers.CustomResourceDefinitionController
}

func validatingWebhookServices(obj *adminregv1.ValidatingWebhookConfiguration) (result []string, _ error) {
	for _, webhook := range obj.Webhooks {
		if webhook.ClientConfig.Service != nil {
			result = append(result, webhook.ClientConfig.Service.Namespace+"/"+webhook.ClientConfig.Service.Name)
		}
	}
	return
}

func crdWebhookServices(obj *apiextv1.CustomResourceDefinition) (result []string, _ error) {
	if obj.Spec.Conversion != nil &&
		obj.Spec.Conversion.Webhook != nil &&
		obj.Spec.Conversion.Webhook.ClientConfig != nil &&
		obj.Spec.Conversion.Webhook.ClientConfig.Service != nil {
		return []string{
			fmt.Sprintf("%s/%s",
				obj.Spec.Conversion.Webhook.ClientConfig.Service.Namespace,
				obj.Spec.Conversion.Webhook.ClientConfig.Service.Name),
		}, nil
	}
	return nil, nil
}

func mutatingWebhookServices(obj *adminregv1.MutatingWebhookConfiguration) (result []string, _ error) {
	for _, webhook := range obj.Webhooks {
		if webhook.ClientConfig.Service != nil {
			result = append(result, webhook.ClientConfig.Service.Namespace+"/"+webhook.ClientConfig.Service.Name)
		}
	}
	return
}

func (h *handler) OnMutationWebhookChange(key string, webhook *adminregv1.MutatingWebhookConfiguration) (*adminregv1.MutatingWebhookConfiguration, error) {
	logrus.Infof("@@@@@OnMutationWebhookChange %s", key)
	if webhook == nil {
		return nil, nil
	}
	for i, webhookConfig := range webhook.Webhooks {
		if webhookConfig.ClientConfig.Service == nil || webhookConfig.ClientConfig.Service.Name == "" {
			continue
		}

		service, err := h.serviceCache.Get(webhookConfig.ClientConfig.Service.Namespace, webhookConfig.ClientConfig.Service.Name)
		if apierror.IsNotFound(err) {
			// OnService will be called when the service is created, which will eventually update the webhook, so no
			// need to enqueue anything if we don't find the service
			return webhook, nil
		} else if err != nil {
			return nil, err
		}

		secret, err := h.generateSecret(service)
		if err != nil {
			return nil, err
		} else if secret == nil {
			continue
		}

		if !bytes.Equal(webhookConfig.ClientConfig.CABundle, secret.Data[corev1.TLSCertKey]) {
			webhook = webhook.DeepCopy()
			webhook.Webhooks[i].ClientConfig.CABundle = secret.Data[corev1.TLSCertKey]
			webhook, err = h.mutatingWebHooks.Update(webhook)
			if err != nil {
				return webhook, err
			}
		}
	}

	return webhook, nil
}

func (h *handler) OnValidatingWebhookChange(key string, webhook *adminregv1.ValidatingWebhookConfiguration) (*adminregv1.ValidatingWebhookConfiguration, error) {
	logrus.Infof("@@@@@OnValidatingWebhookChange %s", key)
	if webhook == nil {
		return nil, nil
	}
	for i, webhookConfig := range webhook.Webhooks {
		if webhookConfig.ClientConfig.Service == nil || webhookConfig.ClientConfig.Service.Name == "" {
			continue
		}

		service, err := h.serviceCache.Get(webhookConfig.ClientConfig.Service.Namespace, webhookConfig.ClientConfig.Service.Name)
		if apierror.IsNotFound(err) {
			// OnService will be called when the service is created, which will eventually update the webhook, so no
			// need to enqueue anything if we don't find the service
			return webhook, nil
		} else if err != nil {
			return nil, err
		}

		secret, err := h.generateSecret(service)
		if err != nil {
			return nil, err
		} else if secret == nil {
			continue
		}

		if !bytes.Equal(webhookConfig.ClientConfig.CABundle, secret.Data[corev1.TLSCertKey]) {
			webhook = webhook.DeepCopy()
			webhook.Webhooks[i].ClientConfig.CABundle = secret.Data[corev1.TLSCertKey]
			webhook, err = h.validatingWebHooks.Update(webhook)
			if err != nil {
				return webhook, err
			}
		}
	}

	return webhook, nil
}

func (h *handler) OnService(key string, service *corev1.Service) (*corev1.Service, error) {
	logrus.Infof("@@@@@OnService %s", key)
	if service == nil {
		return service, nil
	}

	_, err := h.generateSecret(service)
	if err != nil {
		return nil, err
	}

	indexKey := service.Namespace + "/" + service.Name
	mutating, err := h.mutatingWebHooks.Cache().GetByIndex(byServiceIndex, indexKey)
	if err != nil {
		return nil, err
	}
	for _, mutating := range mutating {
		h.mutatingWebHooks.Enqueue(mutating.Name)
	}

	validating, err := h.validatingWebHooks.Cache().GetByIndex(byServiceIndex, indexKey)
	if err != nil {
		return nil, err
	}
	for _, validating := range validating {
		h.validatingWebHooks.Enqueue(validating.Name)
	}

	crd, err := h.crds.Cache().GetByIndex(byServiceIndex, indexKey)
	if err != nil {
		return nil, err
	}
	for _, crd := range crd {
		h.crds.Enqueue(crd.Name)
	}

	return nil, err
}

func (h *handler) OnCRDChange(key string, crd *apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
	logrus.Infof("@@@@@OnCRDChange %s", key)
	if crd == nil || crd.Spec.Conversion == nil || crd.Spec.Conversion.Webhook == nil ||
		crd.Spec.Conversion.Webhook.ClientConfig == nil ||
		crd.Spec.Conversion.Webhook.ClientConfig.Service == nil ||
		crd.Spec.Conversion.Webhook.ClientConfig.Service.Name == "" {
		return crd, nil
	}

	service, err := h.serviceCache.Get(crd.Spec.Conversion.Webhook.ClientConfig.Service.Namespace,
		crd.Spec.Conversion.Webhook.ClientConfig.Service.Name)

	if apierror.IsNotFound(err) {
		// OnService will be called when the service is created, which will eventually update the CRD, so no
		// need to enqueue anything if we don't find the service
		return crd, nil
	} else if err != nil {
		return nil, err
	}

	secret, err := h.generateSecret(service)
	if err != nil || secret == nil {
		return crd, nil
	}

	if !bytes.Equal(crd.Spec.Conversion.Webhook.ClientConfig.CABundle, secret.Data[corev1.TLSCertKey]) {
		crd := crd.DeepCopy()
		crd.Spec.Conversion.Webhook.ClientConfig.CABundle = secret.Data[corev1.TLSCertKey]
		return h.crds.Update(crd)
	}

	return crd, nil
}

func (h *handler) generateSecret(service *corev1.Service) (*corev1.Secret, error) {
	secretName := service.Annotations[SecretAnnotation]
	if secretName == "" {
		return nil, nil
	}

	logrus.Infof("@@@@@Generating secret %s", secretName)
	lockKey := service.Namespace + "/" + service.Name
	h.locker.Lock(lockKey)
	defer h.locker.Unlock(lockKey)

	dnsNameSet := sets.NewString(service.Name+"."+service.Namespace,
		service.Name+"."+service.Namespace+".svc",
		service.Name+"."+service.Namespace+".svc.cluster.local")
	for k, v := range service.Annotations {
		if !strings.HasPrefix(k, DNSAnnotation) {
			continue
		}
		dnsNameSet.Insert(v)
	}

	// this is sorted
	dnsNames := dnsNameSet.List()
	secret, err := h.secretsCache.Get(service.Namespace, secretName)
	if apierror.IsNotFound(err) {
		logrus.Infof("@@@@@Not found in cache.  Calling Creating secret %s", secretName)
		secret, err := h.createSecret(service, service.Namespace, secretName, dnsNames)
		if err != nil {
			logrus.Errorf("@@@@@Failed to create secret %s: %v", secretName, err)
			return nil, err
		}
		return h.secrets.Create(secret)
	} else if err != nil {
		logrus.Errorf("@@@@@Failed to get secret %s: %v", secretName, err)
		return nil, err
	}

	if secret, err := h.updateSecret(service, secret, dnsNames); err != nil {
		logrus.Errorf("@@@@@Failed to update secret %s: %v", secretName, err)
		return nil, err
	} else if secret != nil {
		logrus.Infof("@@@@@In GenerateSecret Updating secret %s", secretName)
		return h.secrets.Update(secret)
	}

	return secret, nil
}

func (h *handler) updateSecret(owner runtime.Object, secret *corev1.Secret, dnsNames []string) (*corev1.Secret, error) {
	tlsCert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil || len(tlsCert.Certificate) == 0 {
		return nil, err
	}

	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, err
	}
	logrus.Infof("@@@@@Checking cert %s for %s/%s", cert.Subject.CommonName, secret.Namespace, secret.Name)
	const (
		oneYear        = 365 * 24 * time.Hour
		refreshMinutes = 10 * time.Minute
	)
	if time.Now().Add(oneYear-refreshMinutes).After(cert.NotAfter) ||
		len(cert.DNSNames) == 0 ||
		!slice.StringsEqual(cert.DNSNames[1:], dnsNames) {
		logrus.Infof("@@@@@Cert %s for %s/%s is expiring or missing DNS names, regenerating", cert.Subject.CommonName, secret.Namespace, secret.Name)
		newSecret, err := h.createSecret(owner, secret.Namespace, secret.Name, dnsNames)
		if err != nil {
			logrus.Errorf("@@@@@Failed to create new secret for %s/%s: %v", secret.Namespace, secret.Name, err)
			return nil, err
		}
		secret = secret.DeepCopy()
		secret.Data = newSecret.Data
		return secret, nil
	}
	logrus.Infof("@@@@@Skipping cert %s for %s/%s", cert.Subject.CommonName, secret.Namespace, secret.Name)
	return nil, nil
}

func (h *handler) createSecret(owner runtime.Object, ns, name string, dnsNames []string) (*corev1.Secret, error) {
	cert, key, err := cert.GenerateSelfSignedCertKey(ns+"-"+name, nil, dnsNames)
	if err != nil {
		return nil, err
	}

	meta, err := meta.Accessor(owner)
	if err != nil {
		return nil, err
	}

	gvk, err := gvk.Get(owner)
	if err != nil {
		return nil, err
	}

	ref := metav1.OwnerReference{
		Name: meta.GetName(),
		UID:  meta.GetUID(),
	}
	ref.APIVersion, ref.Kind = gvk.ToAPIVersionAndKind()

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       ns,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Data: map[string][]byte{
			corev1.TLSCertKey:       cert,
			corev1.TLSPrivateKeyKey: key,
		},
		Type: corev1.SecretTypeTLS,
	}, nil
}
