# Provider v1.0.2: SSH bindings and release

## Authorized result

Publish v1.0.2, explicitly selected by the user, including the native EX coverage
already on main and the missing task SSH bindings. Verify against the immutable
Semaphore EX v2.20.0-ex.2 source (f6e216f7eec730a1fa4fe381e189c9a973ecfa0d).
Use only isolated disposable acceptance fixtures and the existing CI signing
secrets for Registry key 719010B911115D8E. Preserve published versions.

## Implementation sequence

1. Add typed SSH-key bindings and host lists for project default/always policy,
   template overrides and task-start Actions. Use an independently managed
   project policy resource to avoid project/key dependency cycles. Preserve
   unowned settings, imported values and null-versus-empty inheritance.
2. Add focused failing regressions before executable changes, regenerate the
   API client from its specification where needed, and cover lifecycle/import,
   host normalization, invalid references and inherited/empty selections.
3. Pin CI acceptance to the released EX commit; test the complete provider
   against a disposable instance of that server. Run build, lint, generated-doc
   reproducibility and relevant security checks.
4. Document the API/state design in an ADR and usage examples, regenerate docs,
   prepare v1.0.2 release notes, and obtain direct Claude and dedicated Terra
   review before committing.
5. Push verified commits, wait for exact-head CI, tag v1.0.2, and verify the
   signed artifacts, packaged provider startup and public Registry availability.
   Clean task-owned fixtures and retain verification evidence.

## Alternatives

Putting project defaults directly on the project resource creates a dependency
cycle for keys created by the same configuration. A separate policy resource
allows project, key and binding creation in one Terraform apply. Template/run
overrides stay on the objects they configure. Explicit inheritance controls
must coexist with preservation of omitted imported settings; an empty key list
means override-with-none rather than inheritance.
