package constants

// CtxRequestID is the context key under which the per-request ID is
// stored. Used by middlewares.RequestID to attach it, and by
// utils.SuccessJson / utils.ErrorJson to include it in responses.
//
// Declared in the constants package (rather than in middlewares or
// utils) so both can read it without creating an import cycle.
const CtxRequestID = "request_id"
