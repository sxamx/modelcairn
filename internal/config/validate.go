package config

import (
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,128}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Catalog interface {
	Resources() []Resource
	SecretExists(name string) bool
}

func Validate(doc *Document, catalog Catalog) error {
	if doc.APIVersion != APIVersion {
		return failure(CodeInvalidValue, "$.apiVersion")
	}
	if doc.Kind != DocumentKind {
		return failure(CodeInvalidValue, "$.kind")
	}
	if len(doc.Resources) > 10000 {
		return failure(CodeInvalidValue, "$.resources")
	}
	index := map[string]Resource{}
	for i, r := range doc.Resources {
		p := resourcePath(i)
		if !knownKind(r.Kind) {
			return failure(CodeInvalidValue, p+".kind")
		}
		if r.State != Present && r.State != Absent {
			return failure(CodeInvalidValue, p+".state")
		}
		if !namePattern.MatchString(r.Metadata.Name) {
			return failure(CodeInvalidValue, p+".metadata.name")
		}
		if r.Metadata.DisplayName != nil && (utf8.RuneCountInString(*r.Metadata.DisplayName) < 1 || utf8.RuneCountInString(*r.Metadata.DisplayName) > 120) {
			return failure(CodeInvalidValue, p+".metadata.displayName")
		}
		if r.Metadata.Description != nil && utf8.RuneCountInString(*r.Metadata.Description) > 2000 {
			return failure(CodeInvalidValue, p+".metadata.description")
		}
		if r.Metadata.UID != nil && !uuidPattern.MatchString(*r.Metadata.UID) {
			return failure(CodeInvalidValue, p+".metadata.uid")
		}
		if r.Metadata.ResourceVersion != nil && *r.Metadata.ResourceVersion < 1 {
			return failure(CodeInvalidValue, p+".metadata.resourceVersion")
		}
		key := string(r.Kind) + "\x00" + r.Metadata.Name
		if _, ok := index[key]; ok {
			return failure(CodeDuplicateResource, p)
		}
		index[key] = r
		if r.State == Present {
			if err := validateSpec(r, p+".spec"); err != nil {
				return err
			}
		}
	}
	effective := map[string]Resource{}
	if catalog != nil {
		for _, existing := range catalog.Resources() {
			effective[string(existing.Kind)+"\x00"+existing.Metadata.Name] = existing
		}
	}
	for key, desired := range index {
		if desired.State == Absent {
			delete(effective, key)
		} else {
			effective[key] = desired
		}
	}
	lookup := func(kind Kind, name string) (Resource, bool) {
		v, ok := effective[string(kind)+"\x00"+name]
		return v, ok
	}
	secretExists := func(name string) bool { return catalog != nil && catalog.SecretExists(name) }
	for i, r := range doc.Resources {
		if r.State == Absent {
			continue
		}
		p := resourcePath(i) + ".spec"
		if err := validateReferences(r, p, lookup, secretExists, catalog != nil, index); err != nil {
			return err
		}
	}
	if catalog != nil {
		for _, existing := range catalog.Resources() {
			key := string(existing.Kind) + "\x00" + existing.Metadata.Name
			if _, replaced := index[key]; replaced {
				continue
			}
			if err := validateReferences(existing, "$.existing["+string(existing.Kind)+"/"+existing.Metadata.Name+"].spec", lookup, secretExists, true, index); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSpec(r Resource, p string) error {
	switch v := r.Spec.(type) {
	case ProviderSpec:
	case ProviderAccountSpec:
		return validRef(v.ProviderRef, p+".providerRef")
	case ProviderConnectionSpec:
		if err := validRef(v.ProviderRef, p+".providerRef"); err != nil {
			return err
		}
		u, err := url.Parse(v.BaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
			return failure(CodeInvalidValue, p+".baseUrl")
		}
		if !v.AllowPrivateNetwork && isPrivateLiteralHost(u.Hostname()) {
			return failure(CodeInvalidValue, p+".baseUrl")
		}
		if utf8.RuneCountInString(v.BaseURL) > 2048 || v.Adapter != "openai-chat-v1" {
			return failure(CodeInvalidValue, p)
		}
	case CredentialSpec:
		for _, x := range []struct {
			r Ref
			p string
		}{{v.ProviderAccountRef, ".providerAccountRef"}, {v.EgressRef, ".egressRef"}, {v.SecretRef, ".secretRef"}} {
			if err := validRef(x.r, p+x.p); err != nil {
				return err
			}
		}
	case EgressSpec:
		if v.Type != "direct" {
			return failure(CodeInvalidValue, p+".type")
		}
	case ModelSpec:
		if err := validRef(v.ConnectionRef, p+".connectionRef"); err != nil {
			return err
		}
		if utf8.RuneCountInString(v.ProviderModelID) < 1 || utf8.RuneCountInString(v.ProviderModelID) > 255 {
			return failure(CodeInvalidValue, p+".providerModelId")
		}
		allowed := map[string]bool{"text": true, "stream": true, "tools": true, "parallel-tools": true, "developer-role": true, "json-schema": true, "logprobs": true}
		seen := map[string]bool{}
		for i, c := range v.Capabilities {
			if !allowed[c] || seen[c] {
				return failure(CodeInvalidValue, p+".capabilities["+itoa(i)+"]")
			}
			seen[c] = true
		}
	case DestinationSpec:
		if err := validRef(v.ModelRef, p+".modelRef"); err != nil {
			return err
		}
		if err := validRef(v.CredentialRef, p+".credentialRef"); err != nil {
			return err
		}
		if v.Weight < 1 || v.Weight > 1000 {
			return failure(CodeInvalidValue, p+".weight")
		}
	case StrategySpec:
		if len(v.Destinations) < 1 || len(v.Destinations) > 32 {
			return failure(CodeInvalidValue, p+".destinations")
		}
		for i, x := range v.Destinations {
			if err := validRef(x, p+".destinations["+itoa(i)+"]"); err != nil {
				return err
			}
		}
		if v.MaxAttempts < 1 || v.MaxAttempts > 32 || v.AttemptTimeoutMS < 100 || v.AttemptTimeoutMS > 600000 || v.TotalTimeoutMS < 100 || v.TotalTimeoutMS > 900000 {
			return failure(CodeInvalidValue, p)
		}
	case RouteSpec:
		if !aliasPattern.MatchString(v.ModelAlias) {
			return failure(CodeInvalidValue, p+".modelAlias")
		}
		return validRef(v.StrategyRef, p+".strategyRef")
	case AgentTokenSpec:
		if len(v.AllowedRouteRefs) < 1 {
			return failure(CodeInvalidValue, p+".allowedRouteRefs")
		}
		seen := map[string]bool{}
		for i, x := range v.AllowedRouteRefs {
			if err := validRef(x, p+".allowedRouteRefs["+itoa(i)+"]"); err != nil {
				return err
			}
			if seen[x.Name] {
				return failure(CodeInvalidValue, p+".allowedRouteRefs["+itoa(i)+"]")
			}
			seen[x.Name] = true
		}
		if v.ExpiresAt != nil {
			if _, err := time.Parse(time.RFC3339, *v.ExpiresAt); err != nil {
				return failure(CodeInvalidValue, p+".expiresAt")
			}
		}
	default:
		return failure(CodeInvalidStructure, p)
	}
	return nil
}

func validateReferences(r Resource, p string, lookup func(Kind, string) (Resource, bool), secretExists func(string) bool, strict bool, local map[string]Resource) error {
	require := func(kind Kind, ref Ref, path string) (Resource, error) {
		v, ok := lookup(kind, ref.Name)
		_, declaredLocally := local[string(kind)+"\x00"+ref.Name]
		if !ok && (strict || declaredLocally) {
			return Resource{}, failure(CodeReferenceNotFound, path)
		}
		return v, nil
	}
	switch v := r.Spec.(type) {
	case ProviderAccountSpec:
		_, e := require(ProviderKind, v.ProviderRef, p+".providerRef")
		return e
	case ProviderConnectionSpec:
		_, e := require(ProviderKind, v.ProviderRef, p+".providerRef")
		return e
	case CredentialSpec:
		if _, e := require(ProviderAccountKind, v.ProviderAccountRef, p+".providerAccountRef"); e != nil {
			return e
		}
		_, e := require(EgressKind, v.EgressRef, p+".egressRef")
		if e != nil {
			return e
		}
		if strict && !secretExists(v.SecretRef.Name) {
			return failure(CodeReferenceNotFound, p+".secretRef")
		}
		return nil
	case ModelSpec:
		_, e := require(ProviderConnectionKind, v.ConnectionRef, p+".connectionRef")
		return e
	case DestinationSpec:
		model, e := require(ModelKind, v.ModelRef, p+".modelRef")
		if e != nil {
			return e
		}
		credential, e := require(CredentialKind, v.CredentialRef, p+".credentialRef")
		if e != nil {
			return e
		}
		if model.Spec != nil && credential.Spec != nil {
			ms := model.Spec.(ModelSpec)
			cs := credential.Spec.(CredentialSpec)
			connection, _ := lookup(ProviderConnectionKind, ms.ConnectionRef.Name)
			account, _ := lookup(ProviderAccountKind, cs.ProviderAccountRef.Name)
			if connection.Spec != nil && account.Spec != nil && connection.Spec.(ProviderConnectionSpec).ProviderRef.Name != account.Spec.(ProviderAccountSpec).ProviderRef.Name {
				return failure(CodeProviderMismatch, p+".credentialRef")
			}
		}
	case StrategySpec:
		for i, x := range v.Destinations {
			if _, e := require(DestinationKind, x, p+".destinations["+itoa(i)+"]"); e != nil {
				return e
			}
		}
	case RouteSpec:
		_, e := require(StrategyKind, v.StrategyRef, p+".strategyRef")
		return e
	case AgentTokenSpec:
		for i, x := range v.AllowedRouteRefs {
			if _, e := require(RouteKind, x, p+".allowedRouteRefs["+itoa(i)+"]"); e != nil {
				return e
			}
		}
	}
	return nil
}

func validRef(r Ref, p string) error {
	if !namePattern.MatchString(r.Name) {
		return failure(CodeInvalidValue, p+".name")
	}
	return nil
}
func resourcePath(i int) string { return "$.resources[" + itoa(i) + "]" }
func itoa(i int) string {
	const d = "0123456789"
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = d[i%10]
		i /= 10
	}
	return string(b[n:])
}

func isPrivateLiteralHost(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified())
}
