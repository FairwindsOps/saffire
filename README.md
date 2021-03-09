<div align="center" class="no-border">
    <img src="/img/saffire.png" height="150" alt="Saffire" style="padding-bottom: 20px" />
    <br>
    <a href="https://github.com/FairwindsOps/saffire/releases">
        <img src="https://img.shields.io/github/v/release/FairwindsOps/saffire">
    </a>
    <a href="https://goreportcard.com/report/github.com/FairwindsOps/saffire">
        <img src="https://goreportcard.com/badge/github.com/FairwindsOps/saffire">
    </a>
    <a href="https://insights.fairwinds.com/gh/FairwindsOps/saffire">
      <img src="https://insights.fairwinds.com/v0/gh/FairwindsOps/polaris/badge.svg">
    </a>
    <a href="https://join.slack.com/t/fairwindscommunity/shared_invite/zt-e3c6vj4l-3lIH6dvKqzWII5fSSFDi1g">
      <img src="https://img.shields.io/static/v1?label=Slack&message=Join+our+Community&color=%3CCOLOR%3E&logo=slack">
    </a>
</div>

A controller to override image sources in the eventthat an image cannot be pulled.

Built using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)

## Alpha Software

At this time, saffire is currently in _alpha_. This means that we could change literally anything at any time without notice. Keep an eye out for major changes, and hopefully a v1 release at some point.

## Why?

The image repository for docker images is a single point of failure for many clusters. As seen in the past with [rate limiting on Docker Hub]() and several high-profile [Quay.io outages](), these images being unavailable can produce disastrous consequences for Kubernetes cluster operators.

The intent of saffire is to provide operators with a method of automatically switching image repositories when `imagePullErrors` occur.

## How?

This works via controller and a CRD called an `AlternateImageSource`. The `AlternateImageSource` specifies a set of `equivalentRepositories`. These repositories *must* have the exact same image tags pushed to them. In order to achieve this, we recommend pushing images to both repositories from your CI pipeline. Here's an example `AlternateImageSource`

```
apiVersion: saffire.fairwinds.com/v1alpha1
kind: AlternateImageSource
metadata:
  name: alternateimagesource-sample
spec:
  imageSourceReplacements:
    - equivalentRepositories:
        - quay.io/fairwinds/docker-demo
        - ehazlett/docker-demo
```

This indicates that `quay.io/fairwinds/docker-demo` and `ehazlett/docker-demo` have the exact same image tags in both.

Once the controller and this `AlternateImageSource` are installed in your cluster, if any pod experiences an `ImgagePullError` in that namespace and the image matches one of these repositories, saffire will find the top level controller of that pod and patch it to set the image as one of the other repositories in the `equivalentRepositories` field (currently this only applies to deployments).
