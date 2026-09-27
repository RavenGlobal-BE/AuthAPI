package Config

/* This config file includes are changeable settings. They're gonna be hardcoded on compile-time. */

// Program configurations
// Testmode has been migrated to .env (shall no longer be hardcoded)
var AllowVerbose bool = false

// Version & Build
var Version = "26.3"
var Build float32 = 31

// Server configurations
// Port has been migrated to .env

// Feature flags (Beta features, not fully tested, or not implemented yet)
var EnableOpTokens bool = false //In this current implementation, OP tokens can and WILL disrupt & compromise the current auth flow
var AllowQuantumSecure bool = false

// Raven ONE configurations
var CdnEndpoint string = "https://cdn.raven.co.com/orgIcons/"
