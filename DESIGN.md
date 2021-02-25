# POC Design and CRD Organization

## Intended Use

The current intent is that an `alternateImageSource` will be deployed alongside an application. This will specify the possible alternate iamges for that application's pods.

## CRD

The POC or alpha version of saffire has the following CRD Structure for `alternateImageSource` (AIS for short):

```
spec:
  imageSourceReplacements:
    - equivalentRepositories:
        - quay.io/fairwinds/docker-demo
        - ehazlett/docker-demo
```

### imageSourceReplacements

The `imageSourceReplacements` field contains a list. That list has two fields:

#### equivalentRepositories

These are image repositories (`image` field minus tag) that are equivalent. They are expected to have the same image tags. Upon a switch being activated, saffire will loop through these and use the first one that is not currently utilized.

Currently there is no plan to implement a "switch back" functionality. If the end-user desires a switch back, they can re-deploy their manifests with the original image.

## How it Works

In the `SetupWithManager` function, we initiate a pod watcher, that receives all status updates for pods that the controller can access. If the pod has a status `ErrImagePull` or `ImagePullBackOff`, then we initiate a reconciliation of the `alternateImageSources` in that namespace. In addition, we run reconciliation if an AIS is modified or created.

During the reconcilation, if any pods in the namespace of the AIS are experiencing image pull errors, we check to see if they have an image in the equivalentRepositories field. If they do, we trigger a "switch" where the top level controller is looked up, and then patched if possible. SwitchStatuses are added to the AIS at this time.

Switches are only allowed to occur every 30s. This will eventually be moved to a backoff instead.
