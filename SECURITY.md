# Security Policy

## Dependabot Alerts

### GO-2026-5932: golang.org/x/crypto (openpgp)

**Status:** Accepted false positive — no action required.

**Rationale:** The `golang.org/x/crypto` module transitively includes the
abandoned `openpgp` package, which triggers the Dependabot advisory. This
project does **not** import or execute any code from `openpgp`. The only
direct usage of `golang.org/x/crypto` is `bcrypt` for password hashing, which
is unaffected by the reported vulnerability.

**Mitigation:** The Go toolchain does not provide a way to exclude subpackages
from the module graph. Replacing `x/crypto/bcrypt` with an alternative bcrypt
library would add unnecessary risk and maintenance burden. The advisory is
acknowledged as a false positive for this codebase.

**Review date:** 2026-08-19
