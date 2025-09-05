---
name: Enhancement Proposal Template
about: Create a new enhancement proposal
title: "[PROPOSAL] NEW FEATURE"
labels: enhancement
assignees: ''

---

## Abstract

<!-- high level overview of the proposed improvement (how does this add value for users?) --> 

### Background

<!-- provide some background or other context to explain why the enhancement is important --> 

### User Stories

<!-- provide as many user stories as possible to explain how this enhancement benefits users -->

## Proposal

<!-- summary of the proposed changes (list of new/modified features) --> 

### [Feature Enhancement 1]

<!-- example feature enhancement 1; describe how the new/modified feature should work --> 

**Examples:**

<!-- include any monitoring code examples (as appropriate) --> 

```yaml
---
type: NewType
api_version: core/v3
metadata:
  name: example-new-type
spec:
  field1: example
  field2: example
```

### [Feature Enhancement 2]

<!-- example feature enhancement 2 --> 

**Examples:**

<!-- include any monitoring code examples (as appropriate) --> 

```yaml
---
type: NewTypeComponent
api_version: core/v3
metadata:
  name: example-new-type-component
spec:
  field1: example
  field2: example
```

## Implementation Details

<!-- high level overview of implementation details, with emphasis on any implementation details that may require an engineering spike to evaluate --> 

### Documentation Requirements

<!-- does this enhancement require any new or modified documentation? -->

### Upstream Impact(s)

<!-- does this enhancement require any changes to this application/project? --> 

### Downstream Impact(s)

<!-- does this enhancement require any changes for consumers of this application/project (e.g. does it break an API or other contract)? --> 

### Instrumentation/Observability 

<!-- how do we measure the performance of this enhancement? -->
