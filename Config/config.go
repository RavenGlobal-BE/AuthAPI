package Config

/* This config file includes are changeable settings. They're gonna be hardcoded on compile-time. */

// Program configurations
var TestMode bool = false
var AllowVerbose bool = false

// Version & Build
var Version = "26.2.1"
var Build float32 = 24

// Server configurations
var Port int = 3000

// Feature flags (Beta features, not fully tested, or not implemented yet)
var EnableOpTokens bool = false //In this current implementation, OP tokens can and WILL disrupt & compromise the current auth flow
var AllowQuantumSecure bool = false

// Raven ONE configurations
var CdnEndpoint string = "https://cdn.raven.co.com/orgIcons/"
