// Package apperr contiene los errores que comparten varios componentes, para
// que la capa HTTP los traduzca de una sola forma.
package apperr

// ValidationError describe un dato de entrada inválido (respuesta 400).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }
