### NOTE for modifying `registration-service.yaml`
 
After modifying this file remember to:

1) run `make copy-reg-service-template` (which will copy this file locally into ../host-operator/deploy/registration-service/registration-service.yaml)
2) open a PR in [host-operator](https://github.com/codeready-toolchain/host-operator) with that change. This PR needs to be merged before merging the changes in registration-service.

This is required since the actual template used for deploying the registration service is the one present at https://github.com/codeready-toolchain/host-operator/blob/master/deploy/registration-service/registration-service.yaml