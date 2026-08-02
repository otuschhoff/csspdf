# Layered CSS Organization Rules

This document defines how to organize and apply layered CSS in csspdf profiles.

## Purpose

Layered CSS lets teams share common styling while keeping document-specific and customer-specific changes isolated.

Target outcomes:
- stable corporate look across documents
- local document customization without duplication
- deterministic override behavior

## Layer contract

Layers are applied from low precedence to high precedence in declared order.
A later layer can overwrite mapped declarations from earlier layers.

Recommended order:
1. corporate base
2. document layer
3. customer override
4. legacy `Assets.CSS` fallback (implicit final layer when set)

## Directory convention

Recommended layout:
- styles/corporate/base.css
- styles/document/doc.css
- styles/overrides/customer.css

Keep this structure per profile to reduce ambiguity.

## Selector and specificity guidance

- Keep corporate selectors low specificity.
- Put profile-specific overrides in document layer.
- Put customer exceptions only in override layer.
- Avoid high-specificity selectors in early layers.

## Compatibility behavior

- If only legacy `Assets.CSS` is set, behavior is unchanged.
- If `CSSLayers` are set, they are composed in declared order.
- If both `CSSLayers` and legacy `CSS` are set, legacy CSS is appended as an implicit final layer.

## Supported source types for layers

Each `CSSLayerInput.Source` can come from:
- inline text
- file path
- io/fs path

This allows layering from embedded assets, local files, or generated sources.

## Important parser limitations

csspdf does not implement full browser CSS cascade semantics.

Current model:
- only mapped CSS properties are applied
- unknown properties are ignored
- inline attributes win over stylesheet declarations
- `!important` and native browser `@layer` semantics are not interpreted

Treat layered CSS as deterministic mapped-attribute composition rather than full browser rendering.

## End-to-end example profile

See [examples/layered/doc.html](examples/layered/doc.html) with:
- [examples/layered/styles/corporate/base.css](examples/layered/styles/corporate/base.css)
- [examples/layered/styles/document/doc.css](examples/layered/styles/document/doc.css)
- [examples/layered/styles/overrides/customer.css](examples/layered/styles/overrides/customer.css)

Render via CLI:

```bash
go run ./cmd/gen-example layered -o output/layered.pdf
```

## Migration guidance

Suggested migration path from single CSS:
1. move stable shared rules into corporate layer
2. keep existing document rules in document layer
3. add optional customer override layer
4. keep legacy CSS temporarily if needed; remove after parity check

Validate migration by diffing rendered PDFs across representative documents.
