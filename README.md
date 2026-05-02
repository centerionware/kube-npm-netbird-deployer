# quick-npm-deployer

This will include everything required to quickly deploy an npm project from public (maybe private ?) git repositories to kubernetes, create a deployment and a service that's exposed via netbird.


basically a cloudflare worker/vercel highly focused self hosted alternative 

## current status:

* Partially operational, still in development
* operator runs without error
* Build stage runs, images are pushed to local repository 
* Deployment stage runs
* App's run

## todo:

* add cleanup, watch when crd's are removed and remove any associated jobs and deployments and services (Preferably also delete from registry)
* Add registry authentication (optional)
* Add namespace to image name (To prevent collissions if the same application is installed to different namespaces)
* Add ingress & gateway api generators (and cleanup)
* Cleanup build jobs when they complete
* Ensure rebuilds on git changes occur, possibly add canary and rollback functions
  
## Interesting notes

The registry runs inside of kubernetes, buildkit can access it via DNS resolution within the cluster.

However, the part of kubernetes that actually downloads container images is not inside of kubernetes, so it can't resolve the local registry. Using a NodePort service (instead of clusterip) allows all nodes to access the registry via 'localhost:port' (Even if the registry isn't actually running on that specific node, kubernetes will route it from the node's port to the proper pod)
