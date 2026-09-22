package provider

// Keep the known release limitation visible in generated Action references.
const actionWriteOnlyLimitation = "\n\nKnown limitation: HashiCorp Terraform Plugin Framework v1.19.0 rejects non-null write-only Action inputs during plan validation (also reproduced with Terraform 1.16.3). This Action fails before invocation when such inputs are set; required write-only inputs make the Action unavailable. Ordinary resource write-only attributes and ephemeral preview resources are unaffected. The provider retains write-only protection and uses the unmodified official framework."
