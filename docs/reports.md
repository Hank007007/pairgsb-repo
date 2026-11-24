# Violation reporting

CSP violations are collected by a platform service. The edge advertises the
collector through two headers:

* `Content-Security-Policy: ... report-to csp-endpoint` — names the reporting
  group that browsers should use.
* `Reporting-Endpoints: csp-endpoint="https://.../csp/..."` — maps that group
  name to a URL.

## 1. Endpoint rules

A reporting endpoint is accepted only when all of the following hold:

1. **Scheme.** The URL scheme is `https`. `http`, `javascript:`, `data:` and
   every other scheme are rejected.
2. **Registered domain.** The URL host is, or is a subdomain of, a domain
   registered for the tenant on the platform. The registry is
   `src/platform/registry.mjs`. A host that is not registered — including an
   IP literal such as `10.0.0.7`, `localhost`, and another tenant's domain —
   is rejected.
3. **Path prefix.** The URL path starts with `/csp/`. A URL whose path is `/`,
   `/collect`, or anything else is rejected.

Relative URLs are rejected: the endpoint must be absolute so that the value
written into `Reporting-Endpoints` is unambiguous.

## 2. What happens on rejection

A rejected endpoint does not become a header value. The tenant's raw string is
never copied into `Reporting-Endpoints`; the compiler either uses the
platform's default collector or refuses to produce headers for the policy,
depending on the entry point. Either way, the string the tenant typed does not
reach the response.

## 3. Group name

The reporting group name is fixed by the platform (`csp-endpoint`). The
`report-to` directive in the policy names it. A tenant cannot rename the group
or remove the `report-to` directive.
