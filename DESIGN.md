# POC Design and CRD Organization

## Intended Use

The current intent is that an `alternateImageSource` will be deployed alongside an application. This will specify the possible alternate iamges for that application's pods.

## CRD

The POC or alpha version of Kuiper has the following CRD Structure for `alternateImageSource` (AIS for short):

```
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

### imageSourceReplacements

The `imageSourceReplacements` field contains a list. That list has two fields:

#### equivalentRepositories

These are image repositories (`image` field minus tag) that are equivalent. They are expected to have the same image tags. Upon a switch being activated, Kuiper will loop through these and use the first one that is not currently utilized.

Currently there is no plan to implement a "switch back" functionality. If the end-user desires a switch back, they can re-deploy their manifests with the original image.

#### targets

These are a list of `type` and `name` structures that target an individual pod controller. These are what kuiper will look at to possibly replace. Currently, only deployments are supported.

Eventually, I would like to see the ability to use `labelSelectors` instead of only `name`

## How it Works

In the `SetupWithManager` function, we initiate a pod watcher, that receives all status updates for pods that the controller can access. If the pod has a status `ErrImagePull` or `ImagePullBackOff`, then we initiate a reconciliation of the `alternateImageSources` in that namespace. Deployments also trigger a reconciliation, but I'm not sure that's 100% necessary right now. In addition, we run reconciliation if an AIS is modified or created.

During the reconcilation, the `alternateImageSource` checks each of its targets to see if they need to be `activated`. An `activation` is simply switching to a different image source in the list of `equivalentRepositories`. This is done via a patch to the deployment.

Also during the reconciliation, the deployed `alternateImageSource` is updated with the list of potential targets that it has discovered running in the cluster. So if we say "this AIS targets deployment `app`", and the deployment `app` exists in the namespace, it is added to the targets list in the status.

The status of an un-activated deployed AIS looks like:

```
status:
  observedGeneration: 1
  targetsAvailable:
  - container: basic-demo
    currentRepository: quay.io/fairwinds/docker-demo
    name: demo3-basic-demo
    type:
      group: apps
      kind: deployment
    uid: 233ac88f-c125-4125-8788-e95f8531a181
```

If an activation is triggered, the status of the target is updated:

```
status:
  observedGeneration: 1
  targetsAvailable:
  - container: basic-demo
    currentRepository: quay.io/fairwinds/docker-demo
    name: demo3-basic-demo
    switches:
    - newImage: quay.io/fairwinds/docker-demo
      oldImage: ehazlett/docker-demo
      time: "2020-11-30T19:20:18Z"
    type:
      group: apps
      kind: deployment
    uid: 233ac88f-c125-4125-8788-e95f8531a181
```

Note that the `switches` field is added. This contains the repository that was switched, the new one, and the timestamp of that `switch` or `activation`.
