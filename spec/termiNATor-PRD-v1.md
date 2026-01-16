# termiNATor v1 PRD (MVP)

## 0. Document control

* **Product:** termiNATor
* **Doc type:** PRD (MVP v1)
* **Status:** Draft
* **Last updated:** 2026-01-16
* **Owner:** (TBD)

---

## 1. Product summary

**termiNATor** is an in-account, on-demand tool that helps AWS customers identify and quantify **avoidable NAT Gateway spend** caused by workloads in private subnets reaching **AWS services via NAT** when **VPC endpoints** (Gateway endpoints or Interface endpoints/PrivateLink) could be used instead.

### MVP v1 focus (keep it simple, high-confidence)

v1 prioritizes the most common and highest-confidence savings:

* **S3** via **Gateway VPC Endpoint** (cost-effective, typically "free" as an endpoint)
* **DynamoDB** via **Gateway VPC Endpoint** (cost-effective, typically "free" as an endpoint)

v1 supports both:

* **Zonal NAT Gateway** (single-AZ)
* **Regional NAT Gateway** (multi-AZ, AvailabilityMode=regional)

v1 provides two modes:

1. **Quick Win Scan (no Flow Logs):** configuration-only checks and "likely savings" prompts
2. **Deep Dive (short-lived Flow Logs):** quantify observed NAT traffic to S3/DynamoDB and compute avoidable NAT **data processing** charges

---

## 2. Problem statement

### 2.1 Core issue

Private subnet workloads often use **NAT Gateways** for egress. If they access AWS services that support VPC endpoints, routing via NAT can introduce **unnecessary per-GB NAT data processing charges** and, depending on topology, additional **cross-AZ data transfer costs**.

### 2.2 Why customers miss it

* Default routing makes NAT "just work"
* Visibility is low: teams don't know which NAT traffic is destined for AWS services
* Endpoints are perceived as "networking work" (route tables, DNS, policies, multi-AZ questions)
* Cost impact isn't obvious until it becomes large

### 2.3 Customer impact

* Unplanned and avoidable monthly spend
* Difficult prioritization: "What endpoint will save the most?"
* Risk: endpoint changes can be disruptive if rolled out without care (route changes, connection resets)

---

## 3. Goals and non-goals

### 3.1 Goals (v1)

1. **Identify** NAT Gateways (zonal and regional) and the VPCs/subnets they serve.
2. **Detect** missing/misconfigured S3/DynamoDB Gateway endpoints (quick scan).
3. **Quantify** observed NAT traffic to S3/DynamoDB using short-lived Flow Logs (deep dive).
4. **Estimate savings** from reducing NAT **data processing** charges via Gateway endpoints.
5. **Produce actionable output**: per-VPC and per-NAT prioritized recommendations with "how to implement" steps.
6. **Build trust**: explicit run cost controls (duration caps), clear cleanup, and transparent assumptions.

### 3.2 Non-goals (v1)

* Auto-create/modify endpoints or route tables
* Continuous monitoring / alerts
* Multi-account / AWS Organizations rollups
* Full per-service coverage across all AWS services
* Deep workload attribution (pod/task-level mapping for EKS/ECS/Lambda)
* NAT gateway right-sizing or NAT removal recommendations (we may mention it, but not optimize for it)

---

## 4. Target users and use cases

### 4.1 Primary personas

* **FinOps / Cost Engineers:** want fast proof of savings and prioritized actions
* **Platform / DevOps:** want safe, reversible recommendations and rollout guidance
* **Cloud Architects:** want to standardize endpoint usage patterns across VPCs

### 4.2 MVP use cases

* "Show me where NAT is being used for S3/DynamoDB and how much it costs me."
* "Which VPCs are missing gateway endpoints?"
* "We introduced regional NAT gateways; is S3/DynamoDB still hairpinning through NAT?"
* "Give me a short run I can perform today that yields actionable savings."

---

## 5. Customer journey (end-to-end)

### 5.1 Install (one-time)

1. Customer deploys termiNATor via **CloudFormation** (or CDK) in a chosen region.
2. Stack provisions:

   * Orchestrator (Step Functions + Lambda)
   * Minimal storage (S3 bucket for reports)
   * IAM roles and policies (least privilege)
   * Optional: a small DynamoDB table for run metadata (recommended)

### 5.2 Run: Quick Win Scan (default starting point)

1. User starts a "Quick Win Scan" for a selected region.
2. termiNATor:

   * discovers NAT Gateways and their AvailabilityMode (zonal/regional)
   * inventories VPC endpoints (gateway + interface) and route table associations
   * outputs:

     * "Missing S3 gateway endpoint in VPC X"
     * "DynamoDB gateway endpoint exists but not associated with private route tables"
     * "S3 interface endpoint private DNS enabled; verify you are not unintentionally bypassing the gateway endpoint"
3. User can stop here (value in minutes), or proceed to Deep Dive.

### 5.3 Run: Deep Dive (Flow Logs)

1. User selects:

   * one region
   * NAT Gateways to analyze (checkbox list)
   * collection window (default 15 min, max 60 min)
2. termiNATor creates **short-lived VPC Flow Logs** on the selected NAT resources.
3. After the window, termiNATor queries logs, computes bytes to S3/DynamoDB, and produces a report:

   * avoidable NAT **data processing** spend estimate (monthly)
   * recommended endpoint actions and rollout checklist
4. termiNATor cleans up flow logs (and optionally log groups it created).

### 5.4 Implement + validate

1. Customer implements gateway endpoints + route table associations.
2. Customer reruns Deep Dive to confirm NAT bytes to S3/DynamoDB drop.

---

## 6. Functional requirements (MVP v1)

### FR-1: Discovery (NAT gateways, modes, topology)

**Description:** Discover all NAT Gateways in the selected region and gather metadata needed for flow log targeting and reporting.

**Must include:**

* NAT Gateway ID, VPC ID, subnet ID
* ConnectivityType (public/private)
* AvailabilityMode (zonal/regional)
* For zonal NAT: NAT ENI ID (NetworkInterfaceId)
* For regional NAT: identify that flow logs must target RegionalNatGateway resource type

**Acceptance criteria:**

* Lists all NAT gateways and correctly labels zonal vs regional.
* Shows which VPC each NAT belongs to and basic context (subnet, tags).

---

### FR-2: Endpoint inventory (Gateway + Interface endpoints)

**Description:** Inventory endpoints and their associations relevant to S3/DynamoDB (and general hygiene checks).

**Must include:**

* Presence of S3 gateway endpoint and DynamoDB gateway endpoint per VPC
* Route table associations for each gateway endpoint
* Identify private subnet route tables that still default-route to NAT

**Acceptance criteria:**

* Can tell if S3/DynamoDB gateway endpoints are missing.
* Can tell if they exist but are not attached to the route tables used by private subnets.

---

### FR-3: Quick Win Scan findings (no Flow Logs)

**Description:** Produce immediate findings without log collection.

**Minimum findings (v1):**

* Missing S3 gateway endpoint in a VPC that has NAT gateways
* Missing DynamoDB gateway endpoint in a VPC that has NAT gateways
* Endpoint exists but not associated with likely-private route tables (those that route 0.0.0.0/0 to NAT)
* S3 interface endpoint exists with Private DNS enabled (flag as a potential cost-pattern to review if gateway endpoint exists)

**Acceptance criteria:**

* Output includes "what to do next" steps for each finding.
* No resource changes are made in this mode.

---

### FR-4: Flow Logs targeting (zonal + regional NAT)

**Description:** Enable short-lived flow logs for selected NAT resources.

**Implementation rules (v1):**

* **Zonal NAT:** create flow logs on the **NAT ENI** (ResourceType=NetworkInterface, ResourceIds=[eni-…])
* **Regional NAT:** create flow logs on the **NAT Gateway itself** (ResourceType=RegionalNatGateway, ResourceIds=[nat-…])

**Acceptance criteria:**

* Flow logs can be created for both NAT modes when selected.
* If one NAT fails (permissions, unsupported state), the run continues for others and reports partial results.

---

### FR-5: Flow Log configuration (format + aggregation + destination)

**Description:** Use a log format that supports reliable destination attribution and a short aggregation interval.

**v1 defaults:**

* Log destination: **CloudWatch Logs** (single option for MVP)
* Max aggregation interval: **60 seconds**
* Retention: **1 day**
* Custom log format includes at least:

  * `interface-id`, `action`, `start`, `end`, `bytes`
  * `srcaddr`, `dstaddr`
  * `pkt-srcaddr`, `pkt-dstaddr`
  * (optional if available) `pkt-dst-aws-service` for simpler service classification

**Acceptance criteria:**

* Logs contain pkt-level addresses so NAT translation does not break classification.
* Aggregation interval is always 60s unless customer explicitly configures otherwise (not recommended).

---

### FR-6: Run orchestration (async, resilient)

**Description:** Runs exceed Lambda max runtime; orchestration must be async.

**v1 approach:**

* Step Functions state machine orchestrates:

  1. discovery + quick scan
  2. create flow logs (for deep dive)
  3. wait (duration + delivery buffer)
  4. query + analysis
  5. report generation
  6. cleanup

**Acceptance criteria:**

* Run returns a `runId` immediately.
* User can retrieve status: `PENDING`, `COLLECTING`, `ANALYZING`, `COMPLETE`, `FAILED`, `PARTIAL`.
* Cleanup runs even on failure paths (best-effort).

---

### FR-7: Traffic classification (S3/DynamoDB MVP)

**Description:** Identify NAT traffic to S3 and DynamoDB during the capture window.

**Classification rules (v1):**

* Primary method:

  * If field `pkt-dst-aws-service` is present and equals `S3` or `DYNAMODB`, use it.
* Fallback method:

  * Use AWS public IP ranges dataset to map `pkt-dstaddr` and `pkt-srcaddr` to S3/DynamoDB prefixes.
* Count **both directions**:

  * classify flows where `pkt-dstaddr` is S3/DDB (outbound)
  * and where `pkt-srcaddr` is S3/DDB (inbound response)
  * sum bytes for both directions to better approximate NAT "data processed"

**Acceptance criteria:**

* Report includes total bytes and GB for:

  * S3
  * DynamoDB
  * Other/Unclassified
* Report explains the classification method used and confidence.

---

### FR-8: Savings and ROI model (corrected)

**Description:** Provide a credible savings estimate and avoid overpromising.

#### v1 "savings we claim"

* **Avoidable NAT data processing charges** for S3/DynamoDB traffic if gateway endpoints are adopted.

#### v1 "savings we do NOT automatically claim"

* NAT hourly savings (unless the customer removes NAT gateways, which v1 does not recommend/automate)
* Any reductions in cross-AZ data transfer (we may flag potential, but do not model precisely in v1)
* Any reductions in service-side charges (S3 requests, DynamoDB RCUs/WCUs, etc.)

#### Computation (v1)

Inputs:

* Observed bytes to S3/DDB during window (from flow logs)
* Window duration
* NAT data processing price per GB for the region (from Pricing API if enabled; otherwise from a bundled defaults table)

Steps:

1. Convert bytes to GB (GiB or GB; choose one and be consistent; v1 uses GB decimal unless configured)
2. Normalize to "GB per hour" for the capture window
3. Project monthly:
   `ProjectedMonthlyGB = (ObservedGB / ObservedHours) * 730`  (730 hours ~= average month)
4. Estimate avoidable NAT data processing spend:
   `AvoidableNATData$ = ProjectedMonthlyGB * NatDataPricePerGB`
5. Endpoint incremental cost for S3/DDB gateway endpoints:
   `EndpointCost$ = 0` (endpoint itself), plus note "other charges may still apply"

Output:

* Avoidable monthly NAT data processing cost for S3 and for DynamoDB
* Combined "quick win" savings estimate
* Confidence + caveats (sampling window, bursty workloads)

**Acceptance criteria:**

* Report clearly separates:

  * "Avoidable now (data processing)" vs "Potential later (hourly if NAT removed)"
* If Pricing API is not allowed, report is labeled "Estimated using default regional pricing table".

---

### FR-9: Reporting (simple, exportable)

**Description:** Provide outputs useful to both humans and automation.

**v1 outputs:**

* JSON report (canonical)
* CSV summary (one line per NAT, plus totals)
* Markdown summary (optional, for ticketing / PRs)

**Report must include:**

* run metadata: region, start/end, duration, NATs analyzed, mode (quick scan/deep dive)
* findings + prioritized recommendations
* bytes/GB by category (S3, DynamoDB, Other)
* estimated avoidable NAT data processing cost
* "how to implement" checklist for gateway endpoints and route table associations
* warnings/limitations section

**Acceptance criteria:**

* Reports are stored in an S3 bucket created by the stack.
* User can download the report artifacts by `runId`.

---

### FR-10: Cleanup and cost control

**Description:** Ensure the tool is safe and cheap by default.

**v1 rules:**

* Default deep dive duration: 15 minutes
* Max duration: 60 minutes
* Always tag created resources with `CreatedBy=termiNATor`, `RunId=<runId>`
* Cleanup must remove:

  * flow logs created by termiNATor
  * optionally CloudWatch log groups created by termiNATor (or leave with 1-day retention)

**Acceptance criteria:**

* No persistent flow logs are left behind by default.
* On partial failures, tool reports what it could not clean and provides manual cleanup steps.

---

## 7. Non-functional requirements (v1)

### 7.1 Security and privacy

* All processing happens **inside the customer AWS account**
* No packet payloads; only flow log metadata is used
* Encrypt at rest:

  * S3 reports bucket uses SSE-S3 or SSE-KMS (configurable)
  * CloudWatch Logs uses default encryption or KMS if configured
* Principle of least privilege (see IAM section)

### 7.2 Reliability

* Runs are idempotent per `runId`
* Failures degrade gracefully to `PARTIAL` results where possible
* Cleanup executes via "finally" paths

### 7.3 Performance / time-to-value

* Quick Win Scan completes in under ~2 minutes for typical accounts (region scope)
* Deep Dive completes within:

  * duration + delivery buffer + analysis time
  * target: < 30 minutes for default 15-minute window

### 7.4 Cost controls

* Hard cap duration (60 min)
* Optional cap: maximum NAT gateways per run (default 10)
* Flow log retention kept minimal (1 day)

### 7.5 Usability

* Findings are written for non-networking experts:

  * "What we saw"
  * "Why it matters"
  * "What to do"
  * "Risk/impact notes"

---

## 8. IAM policy shape (least privilege outline)

> Note: exact IAM JSON is implementation detail; this section defines the "shape" and scoping principles.

### 8.1 Deployment-time resources (CloudFormation)

Create roles:

* `termiNATor-OrchestratorRole` (Step Functions)
* `termiNATor-WorkerLambdaRole` (Lambda execution)
* `termiNATor-FlowLogsDeliveryRole` (role used by EC2 Flow Logs to publish to CloudWatch Logs)

### 8.2 Read-only permissions (core)

* EC2 read:

  * DescribeNatGateways
  * DescribeVpcs, DescribeSubnets, DescribeRouteTables
  * DescribeVpcEndpoints
  * DescribeNetworkInterfaces
  * DescribeFlowLogs
* CloudWatch metrics read (optional but useful for context):

  * GetMetricData (for NAT gateway bytes metrics if used)
* Pricing read (optional):

  * pricing:GetProducts (or disable and rely on defaults table)

### 8.3 Write permissions (bounded, tagged)

* Flow logs management:

  * CreateFlowLogs
  * DeleteFlowLogs
* CloudWatch Logs:

  * CreateLogGroup (only for termiNATor-managed groups)
  * PutRetentionPolicy
  * DeleteLogGroup (optional)
  * Logs Insights query:

    * StartQuery, GetQueryResults
* S3 reports bucket:

  * PutObject, GetObject, ListBucket on the dedicated report bucket/prefix only
* IAM PassRole:

  * Allow passing only `termiNATor-FlowLogsDeliveryRole` to CreateFlowLogs

### 8.4 Guardrails / conditions

* Require tagging on created resources where supported
* Prefer region scoping:

  * restrict actions to the deployed region where possible
* Explicitly do **not** allow:

  * route table modification
  * endpoint creation
  * NAT gateway modification/deletion

---

## 9. Architecture and data flow (MVP)

### 9.1 Components

* **Entry point:** CLI command or lightweight UI (optional) that triggers a run
* **Orchestrator:** AWS Step Functions state machine
* **Workers:** AWS Lambda functions
* **Data sources:**

  * EC2 APIs (NAT, endpoints, route tables)
  * CloudWatch Logs (flow logs)
  * Optional Pricing API
* **Storage:**

  * S3 bucket for reports
  * Optional DynamoDB table for run metadata/status (recommended)

### 9.2 High-level sequence (Deep Dive)

1. Discover NAT + endpoints + route tables
2. Create flow logs for selected NATs (zonal ENI, regional NAT resource)
3. Wait collection window + buffer
4. Query flow logs, classify traffic to S3/DynamoDB
5. Compute projected monthly avoidable NAT data processing cost
6. Generate report artifacts (JSON/CSV/MD) and write to S3
7. Cleanup flow logs (and optional log groups)
8. Mark run complete

---

## 10. Rollout plan (pragmatic MVP)

### Phase 0: Internal alpha (1–2 customers / internal accounts)

* Validate:

  * zonal NAT path works
  * regional NAT path works
  * log format fields present and classification works
  * cleanup is reliable
* Success criteria:

  * deep dive completes end-to-end with correct artifacts and no orphaned flow logs

### Phase 1: Design partner beta (5–10 customers)

* Add polish:

  * clearer reporting language
  * improved quick scan findings
  * better failure explanations
* Success criteria:

  * at least 3 customers implement S3/DynamoDB gateway endpoints based on report
  * rerun confirms measurable NAT byte reduction for S3/DynamoDB

### Phase 2: Public v1

* Documentation:

  * install + runbook
  * "how to roll out endpoints safely"
* Success criteria:

  * consistent run success rate > 95% (excluding permission-denied)
  * low support burden (common failures have actionable messages)

---

## 11. Risks and mitigations (v1)

1. **Sampling window is not representative**

   * Mitigation: default 15 min, allow reruns, clearly label observed window and confidence

2. **Transit/on-prem/central egress patterns**

   * Mitigation: detect likely non-VPC source CIDRs; warn that gateway endpoints may not help transit traffic

3. **Endpoint rollout disruption**

   * Mitigation: include a "safe rollout checklist" and warn about potential connection resets when routes change

4. **Pricing drift or region differences**

   * Mitigation: Pricing API optional; otherwise ship defaults table and label as estimate

5. **Regional NAT expands over time**

   * Mitigation: report NAT AvailabilityMode and warn that AZ coverage can change; recommend rerun after topology changes

---

## 12. Backlog (explicitly out of scope for v1, but aligned with intent)

* Full **Interface endpoint (PrivateLink) ROI modeling**:

  * hourly per-AZ + per-GB processing, break-even analysis
* Broader AWS service coverage (ECR, STS, KMS, SSM, CloudWatch, Secrets Manager, etc.)
* Workload attribution (EKS pod, ECS task, Lambda function) with optional integrations
* Multi-region and multi-account rollups
* Scheduled runs + trend reporting
* Automated endpoint deployment (opt-in) with change management hooks

---

## Appendix A: "How to implement" checklist (template content for reports)

### For S3 gateway endpoint

* Create a **Gateway VPC endpoint** for S3 in the VPC
* Associate it with the **route tables used by private subnets** that currently route 0.0.0.0/0 to NAT
* Start with a permissive endpoint policy, then tighten later
* Roll out in non-prod first; monitor for connection resets; validate with rerun

### For DynamoDB gateway endpoint

* Same pattern as S3, but for DynamoDB service name
* Confirm route table associations for the private subnets that generate DynamoDB traffic

---
