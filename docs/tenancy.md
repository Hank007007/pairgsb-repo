# Tenancy

tupleresolve is shared by the internal code host and the document system, and
it is a shared service: every request belongs to exactly one tenant.

## 1. One tuple space per tenant

A tenant has:

* its own tuple space — tuples written by tenant A are invisible to tenant B,
  even when the textual tuple is identical (`doc:1#reader@user:alice`);
* its own namespace registry — `doc` in tenant A and `doc` in tenant B are
  independent configurations with independent versions;
* its own data version sequence and zookies.

There is no implicit sharing, no global fallback, and no "public" tenant. If a
request does not name a tenant, the service's configured default tenant is
used; that default is a tenant like any other.

## 2. Namespace ownership

A namespace document belongs to exactly one tenant — the tenant that
registered it. Evaluating a relation on an object whose namespace is not
registered in the caller's tenant is `namespace_not_found`; it is **not**
answered by consulting another tenant's copy.

## 3. Cross-tenant reads are "not found"

A read that references data belonging to another tenant must be treated as if
the data does not exist:

* a check for an object whose namespace does not exist in the caller's tenant
  returns `namespace_not_found`;
* a tuple in the caller's store pointing at an object whose type is not
  registered in the caller's tenant contributes nothing (it cannot be
  evaluated, so it is skipped, exactly as if the tuple were absent);
* a userset subject that cannot be resolved in the caller's tenant contributes
  no members.

The important consequence: a tenant must not be able to observe, through a
boolean or through an `expand` listing, anything about another tenant's data.
Timing and error *code* may differ, but the set of subjects and the membership
answer must be indistinguishable from "that data does not exist here".

## 4. Writes

A write (insert or delete) is only accepted when:

* the object's type has a namespace registered **in the writing tenant**;
* the relation is declared by that namespace;
* for a userset subject, the subject's type also has a namespace in the
  writing tenant and declares the subject's relation;
* the caller is authorized for the tenant (the HTTP layer gets its tenant from
  the authenticated principal; a body-supplied tenant that disagrees with the
  principal is `tenant_mismatch`).

A write that names a namespace from another tenant is rejected with
`tenant_mismatch` (or `namespace_not_found` if the writing tenant has no such
namespace at all). It is never redirected into the other tenant's space.

## 5. Storage layout

Tuples are keyed by `(tenant, object, relation, subject)` — or, equivalently,
each tenant has its own map. A key that omits the tenant cannot distinguish
two tenants' facts and must not be used.

## 6. Zookies and tenants

Zookie tokens are tenant-scoped. A token issued by tenant A is not a valid
`at_least` for tenant B; presenting it is `invalid_request`.
