package kubelego

import (
	"github.com/jetstack/kube-lego/pkg/kubelego_const"

	"fmt"
	"strings"
)

func (kl *KubeLego) TlsIgnoreDuplicatedSecrets(tlsSlice []kubelego.Tls) []kubelego.Tls {

	tlsBySecret := map[string][]kubelego.Tls{}

	for _, elm := range tlsSlice {
		key := fmt.Sprintf(
			"%s/%s",
			elm.SecretMetadata().Namespace,
			elm.SecretMetadata().Name,
		)
		tlsBySecret[key] = append(
			tlsBySecret[key],
			elm,
		)
	}

	output := []kubelego.Tls{}
	for key, slice := range tlsBySecret {
		if len(slice) == 1 {
			output = append(output, slice...)
			continue
		}

		texts := []string{}
		for _, elem := range slice {
			texts = append(
				texts,
				fmt.Sprintf(
					"ingress %s/%s (hosts: %s)",
					elem.IngressMetadata().Namespace,
					elem.IngressMetadata().Name,
					strings.Join(elem.Hosts(), ", "),
				),
			)
		}
		kl.Log().Warnf(
			"the secret %s is used multiple times. These linked TLS ingress elements where ignored: %s",
			key,
			strings.Join(texts, ", "),
		)
	}

	return output
}

func (kl *KubeLego) processProvider(ing kubelego.Ingress) (err error) {

	for providerName, provider := range kl.legoIngressProvider {
		err := provider.Reset()
		if err != nil {
			provider.Log().Error(err)
			continue
		}

		if providerName == ing.IngressProvider() {
			err = provider.Process(ing)
			if err != nil {
				provider.Log().Error(err)
			}
		}

		err = provider.Finalize()
		if err != nil {
			provider.Log().Error(err)
		}
	}
	return nil
}

func (kl *KubeLego) reconfigure(ing kubelego.Ingress) error {
	// setup providers
	kl.processProvider(ing)

	// normify tls config
	// NOTE: this no longer performs a global deduplication
	tlsSlice := kl.TlsIgnoreDuplicatedSecrets(ing.Tls())

	// process certificate validity
	kl.Log().Info("process certificate requests for ingresses")
	errs := kl.TlsProcessHosts(tlsSlice)
	if len(errs) > 0 {
		errsStr := []string{}
		for _, err := range errs {
			errsStr = append(errsStr, fmt.Sprintf("%s", err))
		}
		kl.Log().Error("Error while processing certificate requests: ", strings.Join(errsStr, ", "))
	}

	return nil
}

func (kl *KubeLego) TlsProcessHosts(tlsSlice []kubelego.Tls) []error {
	errs := []error{}
	for _, tlsElem := range tlsSlice {
		err := tlsElem.Process()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
