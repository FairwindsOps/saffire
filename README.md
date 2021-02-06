# Kuiper

A controller to override image sources in the event that an image cannot be pulled.

Built using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)

## Why?

The image repository for docker images is a single point of failure for many clusters. As seen in the past with [rate limiting on Docker Hub]() and several high-profile [Quay.io outages](), these images being unavailable can produce disastrous consequences for Kubernetes cluster operators.

The intent of Kuiper is to provide operators with a method of automatically switching image repositories when `imagePullErrors` occur.

## How?

This works via controller and a CRD called an `AlternateImageSource`. The `AlternateImageSource` specifies a set of `equivalentRepositories`. These repositories *must* have the exact same image tags pushed to them. In order to achieve this, we recommend pushing images to both repositories from your CI pipeline. Here's an example `AlternateImageSource`

```
apiVersion: kuiper.fairwinds.com/v1alpha1
kind: AlternateImageSource
metadata:
  name: alternateimagesource-sample
spec:
  imageSourceReplacements:
    - equivalentRepositories:
        - quay.io/fairwinds/docker-demo
        - ehazlett/docker-demo
      targets:
        - type:
            group: apps
            kind: deployment
          name: demo3-basic-demo
        - type:
            group: apps
            kind: deployment
          name: demo2-basic-demo
```

This indicates that `quay.io/fairwinds/docker-demo` and `ehazlett/docker-demo` have the exact same image tags in both. In this case, we target two different deployments, `demo3-basic-demo` and `demo2-basic-demo`.

Once the controller and this `AlternateImageSource` are installed in your cluster, if either of these two deployments in the targets list experiences an `ImgagePullError`, kuiper will patch the deployment and set the image to use one of the other repositories in the `equivalentRepositories` field.
