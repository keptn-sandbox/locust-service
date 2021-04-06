#!/bin/bash


# 1. setup sockshop and carts as described in keptn tutorials
# https://tutorials.keptn.sh/tutorials/keptn-full-tour-dynatrace-08/index.html

# 2. setup locust service
kubectl apply -f ../deploy/service.yaml

# 3. Disable jmeter-service (in case you don't want to run it in parallel)
kubectl scale deployment/jmeter-service -n "keptn" --replicas=0

# 3. Add a locust test file
keptn add-resource --project=sockshop --service=carts --stage=dev --resource=basic.py --resourceUri=locust/basic.py
keptn add-resource --project=sockshop --service=carts --stage=staging --resource=load.py --resourceUri=locust/load.py
keptn add-resource --project=sockshop --service=carts --stage=production --resource=health.py --resourceUri=locust/health.py
keptn add-resource --project=sockshop --service=carts --stage=dev --resource=locust.conf.yaml --resourceUri=locust/locust.conf.yaml

# 4. Trigger a delivery
keptn trigger delivery --project=sockshop --service=carts --image=docker.io/keptnexamples/carts --tag=0.12.3
