# Block Multitenant Resources <small>[MTB-PL1-BC-CPI-2] </small>
**Profile Applicability:** 
1 <br>
**Type:** 
Behavioral <br>
**Category:** 
Tenant Isolation <br>
**Description:** 
Each tenant namespace may contain resources setup by the cluster administrator for multi-tenancy, such as role bindings, and network policies. Tenants should not be allowed to modify the namespaced resources created by the cluster administrator for multi-tenancy. However, for some resources such as network policies, tenants can configure additional instances of the resource for their workloads. <br>
**Remediation:**
 <br>

**audit:** 
The resources managed by the cluster administrator and that cannot be modified by tenant administrator can be identified by a label configured in the benchmarks configuration YAML file. If no label is provided, then this test looks for any existing network policy and role binding (resource quotas are handled by a separate test) and tries to modify and delete them. Run the following commands to retrieve the list of resources managed by the cluster administrator
```bash
kubectl --kubeconfig=cluster-admin -n a1 get all -l =
```
For each returned by the first command verify that the resource cannot be modified by the tenant administrator: kubectl --dry-run=true --kubeconfig=tenant-a -n a1 annotate key1=value1

Each command must return 403 FORBIDDEN <br>

**rationale:** 
Tenants can escalate priviliges and impact other tenants if they are able to delete or modify required multi-tenancy resources such as namespace resource quotas or default network policy. <br>

