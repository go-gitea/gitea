---
aliases: infraql
created_by: Jeffrey Aven and Kieran Rimmer
display_name: StackQL
released: July 2021
short_description: StackQL allows users to query and interact with cloud resources in real-time using SQL.
topic: stackql
url: https://stackql.io/
logo: stackql.png
---
StackQL is an open source tool enabling developers and analysts to query and interact with cloud and SaaS services using a SQL grammar and an ORM that maps to the particular cloud or SaaS provider.  

StackQL includes a SQL parser, DAG planner, and executor, which transpiles SQL statements to provider API requests, returning responses as tabular data (with support for complex objects and arrays).  

StackQL can be used for cloud/SaaS inventory, reporting, and analysis, with the ability to correlate data across cloud providers using SQL `JOIN` operations.  

StackQL can also be used for traditional infrastructure-as-code operations such as scaffolding, provisioning, and de-provisioning, as well as lifecycle operations such as starting or stopping VM instances, activating/deactivating users, etc.  StackQL routines can source configuration data or variables from `json` or [`jsonnet`](https://github.com/google/jsonnet) files.  

StackQL providers are defined by extensions to the providers OpenAPI specification, more information can be found at [stackql/stackql-provider-registry](https://github.com/stackql/stackql-provider-registry) and [stackql/openapi-doc-util](https://github.com/stackql/openapi-doc-util).  
