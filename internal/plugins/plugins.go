// Package plugins re-exporta el contrato del módulo privado
// github.com/paulomcnally/p40la-ihost-automation-plugins para que services y
// api lo consuman sin cambiar imports. Las implementaciones viven en ese repo
// privado (SPEC-015).
package plugins

import (
	pplugins "github.com/paulomcnally/p40la-ihost-automation-plugins/plugins"
)

type (
	// Plugin define el contrato común que todo plugin de integración debe cumplir.
	Plugin = pplugins.Plugin
	// Bill representa una factura/pago obtenida del proveedor.
	Bill = pplugins.Bill
	// CredentialField describe un campo del JSON de credenciales de una cuenta.
	CredentialField = pplugins.CredentialField
	// Registry mantiene los plugins disponibles por nombre.
	Registry = pplugins.Registry
)

// NewRegistry crea un Registry vacío.
var NewRegistry = pplugins.NewRegistry

// Errores de dominio del sistema de plugins.
var (
	ErrAuthFailed       = pplugins.ErrAuthFailed
	ErrAuthExpired      = pplugins.ErrAuthExpired
	ErrServiceNotFound  = pplugins.ErrServiceNotFound
	ErrAPISchemaChanged = pplugins.ErrAPISchemaChanged
	ErrUpstream         = pplugins.ErrUpstream
)