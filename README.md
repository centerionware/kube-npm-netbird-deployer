# quick-npm-deployer

This will include everything required to quickly deploy an npm project from public (maybe private ?) git repositories to kubernetes, create a deployment and a service that's exposed via netbird.


basically a cloudflare worker/vercel highly focused self hosted alternative 

current status:
* not working, in development
* operator runs without error but does not progress through the stages
* git is implemented wrong, tries to use git cli app inside a from scratch container that only contains operator binary
* logging needs to be extended and add logging configuration environment variable
