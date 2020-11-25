# Kuiper

A controller to override image sources in the event that an image cannot be pulled.

Built using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)

## Why?

The image repository for docker images is a single point of failure for many clusters. As seen in the past with [rate limiting on Docker Hub]() and several high-profile [Quay.io outages](), these images being unavailable can produce disastrous consequences for Kubernetes cluster operators.

The intent of Kuiper is to provide operators with a method of 
