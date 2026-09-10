package adminsettings

import "encoding/json"

func ResolveInitial(document *Document) (Resolved, error) {
	if document == nil {
		return Resolved{}, failure(CodeInvalidStructure, "$")
	}
	if document.ResourceVersion != nil {
		return Resolved{}, failure(CodeInvalidStructure, "$.resourceVersion")
	}
	if document.Spec.PublicOrigin == nil {
		return Resolved{}, failure(CodeInvalidStructure, "$.spec.publicOrigin")
	}
	if err := validatePatchStructure(document.Spec); err != nil {
		return Resolved{}, err
	}
	result := apply(Defaults(), document.Spec)
	if normalized, err := normalizePublicOrigin(result.PublicOrigin); err == nil {
		result.PublicOrigin = normalized
	}
	return result, Validate(result)
}

func ResolveUpdate(document *Document, current Resolved) (Resolved, error) {
	if document == nil || document.ResourceVersion == nil || *document.ResourceVersion < 1 {
		return Resolved{}, failure(CodeInvalidStructure, "$.resourceVersion")
	}
	if err := validatePatchStructure(document.Spec); err != nil {
		return Resolved{}, err
	}
	result := apply(current, document.Spec)
	if normalized, err := normalizePublicOrigin(result.PublicOrigin); err == nil {
		result.PublicOrigin = normalized
	}
	return result, Validate(result)
}

func CanonicalJSON(value Resolved) ([]byte, error) { return json.Marshal(value) }

func apply(v Resolved, p Patch) Resolved {
	v.TrustedProxyCIDRs = append([]string{}, v.TrustedProxyCIDRs...)
	if p.PublicOrigin != nil {
		v.PublicOrigin = *p.PublicOrigin
	}
	if p.Listen != nil {
		v.Listen = *p.Listen
	}
	if p.Transport != nil {
		v.Transport = *p.Transport
	}
	if p.TrustedProxyCIDRs != nil {
		v.TrustedProxyCIDRs = append([]string{}, (*p.TrustedProxyCIDRs)...)
	}
	if p.TLSCertificatePath != nil {
		v.TLSCertificatePath = *p.TLSCertificatePath
	}
	if p.TLSPrivateKeyPath != nil {
		v.TLSPrivateKeyPath = *p.TLSPrivateKeyPath
	}
	if p.IdleSeconds != nil {
		v.IdleSeconds = *p.IdleSeconds
	}
	if p.AbsoluteSeconds != nil {
		v.AbsoluteSeconds = *p.AbsoluteSeconds
	}
	if p.GlobalAttemptsPerMinute != nil {
		v.GlobalAttemptsPerMinute = *p.GlobalAttemptsPerMinute
	}
	if p.GlobalBurst != nil {
		v.GlobalBurst = *p.GlobalBurst
	}
	if p.ClientAttemptsPerMinute != nil {
		v.ClientAttemptsPerMinute = *p.ClientAttemptsPerMinute
	}
	if p.ClientBurst != nil {
		v.ClientBurst = *p.ClientBurst
	}
	if p.MaxClientEntries != nil {
		v.MaxClientEntries = *p.MaxClientEntries
	}
	if p.ClientIdleSeconds != nil {
		v.ClientIdleSeconds = *p.ClientIdleSeconds
	}
	if p.ArgonMemoryKiB != nil {
		v.ArgonMemoryKiB = *p.ArgonMemoryKiB
	}
	if p.ArgonIterations != nil {
		v.ArgonIterations = *p.ArgonIterations
	}
	return v
}
