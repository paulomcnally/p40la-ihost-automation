package models

// Setting representa un par clave/valor de configuración persistente.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
