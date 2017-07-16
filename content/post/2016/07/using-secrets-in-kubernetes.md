---
title: "Using Secrets in Kubernetes"
date: 2016-07-16
draft: false
author: "Thomas Mullaly"
tags: ["container-engine", "kubernetes"]
categories: ["coding"]
---

In my previous [blog post]({{< relref "post/2016/06/host-ghost-using-container-engine.md" >}}) I detailed how to setup Ghost and MySQL using Google Cloud Container Engine (GKE) and Kubernetes. While the set up works great the database credentials were hard-coded and completely visible to anyone that has access to the repository. Ideally you would not want these credentials stored as clear text in your source control (especially production credentials). To help facilitate this, Kubernetes has the concept of a Secret. Secrets allow small amounts of sensitive data (tokens, passwords, credentials) to be stored as objects in the cluster. Since Secrets are stored in the cluster it allows for greater control over who has access to them. I highly recommend reading the [documentation](http://kubernetes.io/docs/user-guide/secrets/) on what Secrets are and how they work. The ideal candidate for using a Secret in this blog set up is for the credentials to the MySQL database.

## Setting Up the MySQL Secret

Setting up a Secret can be done in one of two ways:

1. Just like every other Kubernetes component, defining a yaml (or json) config file using the [Secret spec](http://kubernetes.io/docs/user-guide/secrets/#creating-a-secret-manually).
2. Generating secrets using the [command line](http://kubernetes.io/docs/user-guide/secrets/#creating-a-secret-using-kubectl-create-secret).

For this setup, I'll stick with the easier of the two, which is to define the Secret in yaml format.

###### mysql-secrets.yaml

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysql-secrets
type: Opaque
data:
  mysql-root-password: <base64 root password>
  mysql-user: <base64 username>
  mysql-password: <base64 password>
```

Here we define a Secret with a `name` of `mysql-secrets`. This name is how the Secret can be referenced by Pods that need to use the credentials from it. The `data` section is where we define the credentials that want to be apart of this Secret. One thing to note about Secrets is that all values in the Secret are required to be base64 encoded. When Secrets are consumed by Pods running in the cluster, Kubernetes will automatically decode the values for you. If you're running macOS or Linux, you can base64 encode values easily from the terminal:

```bash
$ echo "your value" | base64
eW91ciB2YWx1ZQo=
```

Once you have encoded all the database credentials that you used from my [last blog post](https://blog.tmullaly.com/2016/06/15/hosting-ghost-blog-using-google-cloud-container-engine/), you can create the Secret on the cluster. To do this you run the following command:

```bash
$ kubectl create -f mysql-secrets.yaml
```

## Using the MySQL Secrets

Now that the Secret has been created on the cluster, we can start consuming the values from our Pods. Kubernetes provides a [couple of options](http://kubernetes.io/docs/user-guide/secrets/#using-secrets) for using Secrets from inside of Pods. For this set up, we'll be pulling the Secrets in as [environment variables](http://kubernetes.io/docs/user-guide/secrets/#using-secrets-as-environment-variables). To do this will require making modifications to MySQL and Ghost Deployment specs that we set up previously.

###### mysql-deployment.yaml

```yaml
- image: mysql:5.6
  name: mysql-container
  env:
  - name: MYSQL_ROOT_PASSWORD
    valueFrom:
      secretKeyRef:
        name: mysql-secrets
        key: mysql-root-password
  - name: MYSQL_USER
    valueFrom:
      secretKeyRef:
        name: mysql-secrets
        key: mysql-user
  - name: MYSQL_PASSWORD
    valueFrom:
      secretKeyRef:
        name: mysql-secrets
        key: mysql-password
```

Instead of defining the values for these environment variables inline we now use the `valueFrom` and `secretKeyRef` constructs. These constructs allows us to reference a Secret by `name` (`mysql-secrets`) and to reference a specific key from the Secret. We also make the same changes to the Ghost Deployment.

###### ghost-deployment.yaml

```yaml
- name: DB_USER
  valueFrom:
      secretKeyRef:
        name: mysql-secrets
        key: mysql-user
- name: DB_PASSWORD
  valueFrom:
      secretKeyRef:
        name: mysql-secrets
        key: mysql-password
- name: DB_NAME
  valueFrom:
      secretKeyRef:
        name: mysql-secrets
        key: mysql-password
```

Now that we've changed the Deployments, we can push the changes to the cluster:

```bash
$ kubectl apply -f mysql-deployment.yaml
$ kubectl apply -f ghost-deployment.yaml
```

And that's it. The MySQL an Ghost Pods are now pulling the MySQL credentials from the `mysql-secrets` Secret.

## Wrap up

Kubernetes Secrets are a great way of managing credentials in a cluster. They allow sensitive credentials to be stored and easily consumed from inside of the cluster. You can check out the the full code example [here](https://github.com/thomas-mullaly/ghost-mysql-gke).