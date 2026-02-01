# 🎯 termiNATor - Help Me Break This!

Hey team! I built a tool to find hidden NAT Gateway costs and I need you to **grill it** 🔥

## The Problem (Why You Should Care)

**NAT Gateways charge $0.045/GB** for data processing. If your apps access S3 or DynamoDB through NAT, you're paying for traffic that should be **FREE** with VPC Gateway Endpoints.

**Real example from our test:**
- Traffic analyzed: 5.4 TB/month
- Current NAT cost: **$2M/month** 💸
- Potential savings: **$977K/month** with free VPC endpoints
- **That's $11.7M/year!**

## Try It (2 Minutes)

### Quick Install
```bash
# macOS (Apple Silicon)
curl -L https://github.com/eranchetz/termiNAT/releases/download/v0.0.1/terminat-darwin-arm64 -o terminat
chmod +x terminator

# macOS (Intel)
curl -L https://github.com/eranchetz/termiNAT/releases/download/v0.0.1/terminat-darwin-amd64 -o terminat
chmod +x terminator

# Linux
curl -L https://github.com/eranchetz/termiNAT/releases/download/v0.0.1/terminat-linux-amd64 -o terminat
chmod +x terminator
```

### Run It (Instant Results)
```bash
# Set your AWS profile
export AWS_PROFILE=your-profile
export AWS_REGION=us-east-1

# Quick scan (instant, read-only, no resources created)
./terminat scan quick --region us-east-1
```

**That's it!** You'll see:
- Which NAT Gateways you have
- Missing VPC endpoints (S3, DynamoDB)
- Exact AWS CLI commands to fix it

## Want to See Real Savings? (10 Minutes)

```bash
# Deep dive scan (creates temporary Flow Logs, analyzes real traffic)
./terminat scan deep --region us-east-1 --duration 5
```

**What happens:**
1. Asks your permission before creating anything
2. Creates temporary Flow Logs (you approve first)
3. Waits 5 min for Flow Logs to initialize
4. Collects 5 min of real traffic
5. Shows you:
   - How much S3 traffic goes through NAT (wasted $$$)
   - How much DynamoDB traffic goes through NAT (wasted $$$)
   - Exact monthly/annual savings
   - Which IPs/instances are the biggest offenders
6. **Automatically cleans up** Flow Logs

**Cost to run:** < $0.10 (Flow Logs ingestion)

## What I Need From You 🙏

### 1. Break It
- Run it in your AWS accounts
- Try weird edge cases
- Find bugs, crashes, confusing output
- Test on different regions

### 2. Question Everything
- Are the cost estimates accurate?
- Is the output clear?
- Are the recommendations actionable?
- Does it handle errors gracefully?

### 3. UX Feedback
- Is it too slow?
- Too verbose? Not enough info?
- Confusing prompts?
- Missing features?

### 4. Documentation
- Is [USAGE.md](https://github.com/eranchetz/termiNAT/blob/main/USAGE.md) clear?
- Can you follow it without asking me questions?
- What's missing?

## Known Limitations (Don't Waste Time on These)

- ⚠️ Only analyzes S3 and DynamoDB (not other services yet)
- ⚠️ Cost estimates are projections based on traffic samples
- ⚠️ Requires IAM permissions (documented in USAGE.md)
- ⚠️ Flow Logs need 5 minutes to initialize (AWS limitation)

## Quick Demo Video (If You Want)

**Quick Scan Demo:**
```bash
# Shows instant results in < 1 second
./terminat scan quick --region us-east-1
```

**Deep Dive Demo:**
```bash
# Shows real-time progress with countdown timer
./terminat scan deep --region us-east-1 --duration 5
```

## Interesting Test Scenarios

### Scenario 1: "I have no NAT Gateways"
- What does it say?
- Is the message helpful?

### Scenario 2: "I already have VPC endpoints"
- Does it detect them correctly?
- Does it still recommend creating them?

### Scenario 3: "I have multiple NAT Gateways"
- Does it analyze all of them?
- Can I target a specific one?

### Scenario 4: "My traffic is mostly to EC2/RDS, not S3"
- What does the report show?
- Are the savings realistic?

### Scenario 5: "I interrupt the scan (Ctrl+C)"
- Does it clean up Flow Logs?
- Does it leave orphaned resources?

### Scenario 6: "I run it in a region with no resources"
- Does it handle gracefully?
- Is the error message clear?

## What Success Looks Like

**For me:**
- You find bugs I can fix
- You suggest features that make sense
- You confirm the cost estimates are reasonable
- You think customers would actually use this

**For you:**
- You learn something about NAT Gateway costs
- You potentially save money in your own accounts
- You get to break something before customers do 😈

## Resources

- **GitHub**: https://github.com/eranchetz/termiNAT
- **Usage Guide**: https://github.com/eranchetz/termiNAT/blob/main/USAGE.md
- **E2E Testing**: https://github.com/eranchetz/termiNAT/blob/main/E2E_TESTING.md

## Feedback

**Slack me** or **open GitHub issues**: https://github.com/eranchetz/termiNAT/issues

Be brutal. I want this to be bulletproof before we show customers.

---

**TL;DR:**
1. Install: `curl -L https://github.com/eranchetz/termiNAT/releases/download/v0.0.1/terminat-darwin-arm64 -o terminat && chmod +x terminator`
2. Run: `./terminat scan quick --region us-east-1`
3. Break it and tell me what's wrong

Thanks! 🙏
