#!/bin/bash

# kubebuilder init --domain meridian.io --repo github.com/aoxn/meridian

for tool in "Machine" "Nodepool" "NodepoolComponentConfig" "MachineTask";
do
        echo "Create CRD:" [$crd]
        echo kubebuilder create api --group meridian --version v1 --kind $crd --force true --namespaced false
done


kubebuilder create webhook --group meridian --version v1 --kind MasterSet --defaulting --programmatic-validation --conversion
kubebuilder create webhook --group meridian --version v1 --kind Infra --defaulting --programmatic-validation --conversion
kubebuilder create webhook --group meridian --version v1 --kind Cluster --defaulting --programmatic-validation --conversion

