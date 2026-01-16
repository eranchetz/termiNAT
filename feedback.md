# termiNATor Feedback

## Completed ✅

- [x] While waiting and scanning the flowlogs, we must have a better way to show the customer the progress, the current UX is not good enough it's too static. showing some hints to user about what exactly is happning is also good practice.
  - Added progress bar with percentage
  - Added elapsed/remaining time display
  - Added rotating tips during waiting phases
  - Added phase-specific status messages

- [x] The report is very not clear, it should state exactly which instances are sending traffic to s3 or dynamoDB (and other resources in the future) thorugh the NAT GW, and it should give explicit explanation on how to solve that.
  - Added source IP tracking in traffic analysis
  - Shows top source IPs with traffic breakdown
  - Added explicit AWS CLI commands for creating VPC endpoints
  - Added clear remediation steps section

- [x] The scanner is creating a flowlog and cloudwatch log group, this has to be approved by the user, you should not do anything that incurs cost without explicit user approval
  - Added approval prompt before creating resources
  - Shows exactly what will be created
  - Shows estimated costs
  - Requires explicit Y/n confirmation

- [x] terminNATor should show that it has expliclty stopped the flow logs and ask customer approval to delete the cloudwatch logs and other related resources that were created by it to not incur any costs
  - Shows "✓ Flow Logs STOPPED" message
  - Shows traffic summary before cleanup
  - Asks for explicit approval to delete CloudWatch logs
  - User can choose to keep logs for further analysis

## Future Improvements

- [ ] Add --yes flag to skip approval prompts for automation
- [ ] Add JSON output format for programmatic consumption
- [ ] Support multiple NAT Gateways in parallel
- [ ] Add cost comparison with Interface VPC Endpoints
- [ ] Show ENI-to-instance mapping (requires additional EC2 API calls)
