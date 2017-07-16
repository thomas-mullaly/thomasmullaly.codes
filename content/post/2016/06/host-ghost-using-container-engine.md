---
title: "Hosting Ghost Using Container Engine"
date: 2016-06-15
draft: false
author: "Thomas Mullaly"
tags: ["container-engine","kubernetes","ghost","mysql"]
categories: ["coding"]
---

This is the first post in a series that will document how I got my [Ghost blog](https://ghost.org/) up and running using [Google Cloud Container Engine](https://cloud.google.com/container-engine/) (GKE). By the end of this, you will have a basic (but useable) blog set up that is publicly accessible. This post is going to follow along pretty closely with Google's tutorial for [hosting a Wordpress blog](https://cloud.google.com/container-engine/docs/tutorials/persistent-disk/) using GKE. I deviate from the Wordpress tutorial in a few key areas:

1. Instead of using a Wordpress docker image I am using the [Ghost docker image](https://hub.docker.com/_/ghost/) (naturally).
2. Instead of doing a manual set up of the Google Cloud project environment I am using [Google Cloud Deployment Manager](https://cloud.google.com/deployment-manager/) to automatically provision the cluster and persistent disks.
3. Instead of deploying the Ghost and MySQL pod directly to the cluster I am using [Deployments](http://kubernetes.io/docs/user-guide/deployments/) to manage the pods.

I should note that this certainly is not the most cost effective set up for your personal blog. My intentions for this set up was simply to learn more about [Google Cloud](https://cloud.google.com/), [Kubernetes](http://kubernetes.io/), [Docker](https://www.docker.com/), and [Ghost](https://ghost.org/).

## Prerequisites

Before you get started with the rest of this blog post you are going to need a few components installed. If you have already used GKE before, you can keep on scrolling past this section.

The first component you are going to need to install is the Google Cloud SDK. This SDK comes with the `gcloud` CLI tool, which will allow you to interact with your Google Cloud project using your terminal. This tool will also allow you to install `kubectl` on your machine, which is how you will run commands on your GKE cluster. There is a great [quick start guide](https://cloud.google.com/container-engine/docs/quickstart#install_the_gcloud_command-line_interface) available which will help you get your machine set up.

Once you have `gcloud` and `kubectl` configured on your machine, you will need to make sure you have [initialized](https://cloud.google.com/sdk/docs/quickstart-linux#initialize_the_sdk) `gcloud` so that it has access to your Google Cloud project.

## Deployment Manager Configuration

If you are not familiar with Deployment Manager think infrastructure as code. You write a configuration file (yaml) where you describe the desired state of your cloud environment (ie: VM instances, disks, load balancer, firewall rules, etc.). When this configuration is run through Deployment Manager, it will automatically create (or update) the resources that are defined. This is a huge time saver for large, complex environment set ups, as the entire process can be automated in a completely reproducible fashion.

The deployment for this blog set up will provision a GKE cluster and two persistent disks. The cluster is what MySQL and Ghost Docker containers will eventually be running on. The persistent disks are used for storing Ghost and MySQL data, so that it survives either of those Pods being terminated (either expectedly or unexpectedly).

To help with reusability, the configuration has been split out into two [template](https://cloud.google.com/deployment-manager/configuration/adding-templates) files. The first one is a template for creating a GKE cluster and the second is for creating a persistent disk. To start things off, lets take a look at the cluster template.

###### cluster.jinja

```yaml
{% set CLUSTER_NAME = env['deployment'] + '-' + env['name'] %}

resources:
- name: {{ CLUSTER_NAME }}
  type: container.v1.cluster
  properties:
    zone: {{ properties['zone'] }}
    cluster:
      name: {{ CLUSTER_NAME }}
      initialNodeCount: {{ properties['initialNodeCount'] }}
      nodeConfig:
        machineType: {{ properties['machineType'] }}
        diskSizeGb: {{ properties['diskSizeGb'] }}
        oauthScopes:
        - https://www.googleapis.com/auth/compute
        - https://www.googleapis.com/auth/devstorage.read_only
        - https://www.googleapis.com/auth/logging.write
        - https://www.googleapis.com/auth/monitoring
      masterAuth:
        username: {{ properties['username'] }}
        password: {{ properties['password'] }}
```

The first line declares a variable `CLUSTER_NAME` which is computed based on the name of the Deployment Manager deployment and the name of cluster resource as defined in the configuration file (more on this later). The `type` of resource that is being created is `container.v1.cluster`. This tells Deployment Manager that it needs to provision a GKE cluster.

The `properties:` section is where we can customize the GKE cluster. Using [template variables](https://cloud.google.com/deployment-manager/configuration/adding-templates#template_variables) this template allows values to be defined in the configuration, instead of being hardcoded in the template. The template consumes those variables using the `{{ properties['theProperty'] }}` syntax. This template allows the following cluster properties to be overridden: `zone`, `initialNodeCount`, `machineType`, `diskSizeGb`, `username`, and `password`. These properties will make more sense when we go over the deployment configuration.

The `oauthScopes` indicate what permissions the nodes in the GKE cluster have. General rule of thumb is to grant the least amount of permissions required to run apps in the cluster. The permissions being granted in this template are:

* `https://www.googleapis.com/auth/compute` - Full access to the compute engine API. This lets Kubernetes mount persistent disks to the Pods it's running.
* `https://www.googleapis.com/auth/devstorage.read_only` - Read-only access to the projects [Storage Buckets](https://cloud.google.com/storage/). Kubernetes will need this to download Docker images from the projects private [Container Registry](https://cloud.google.com/container-registry/).
* `https://www.googleapis.com/auth/logging.write` - Write-only access to the [Stackdriver Logging](https://cloud.google.com/logging/) APIs. This will allow you view any logs that are generated by Kubernetes or the Pods that Kubernetes is running.
* `https://www.googleapis.com/auth/monitoring` - Full-access to the [Stackdriver Monitoring](https://cloud.google.com/monitoring/) APIs. This lets you view resource usage via the Stackdriver Monitoring console.

###### persistentDisk.jinja

```yaml
{% set DISK_NAME = env['deployment'] + "-disk-" + env['name'] %}

resources:
- name: {{ DISK_NAME }}
  type: compute.v1.disk
  properties:
    zone: {{ properties["zone"] }}
    sizeGb: {{ properties["sizeGb"] }}
    type: https://www.googleapis.com/compute/v1/projects/{{ env["project"] }}/zones/{{ properties["zone"] }}/diskTypes/{{ properties["diskType"] }}
```

Much like the cluster template, the first line declares a computed variable `DISK_NAME`. This name is based on the name of the deployment and the name of the disk resource as defined in the configuration template. This will ultimately be the name of the persistent disk as it appears in Google Cloud. The `type` of resource that is being created is `compute.v1.disk`.

This template allows for some customization of the [disk](https://cloud.google.com/compute/docs/disks/#pdspecs), specifically `zone`, `sizeGb`, and `diskType` via template variables. The `type` property uses a URL to describe what [disk type](https://cloud.google.com/compute/docs/disks/#introduction) to provision.

The final piece of this deployment is the actual configuration file.

###### cluster.yaml

```yaml
imports:
- path: persistentDisk.jinja
- path: cluster.jinja

resources:
- name: blog-cluster
  type: cluster.jinja
  properties:
    zone: us-east1-c
    username: <your cluster username>
    password: <your cluster password>
    initialNodeCount: 2
    diskSizeGb: 50
    machineType: g1-small
- name: ghost
  type: persistentDisk.jinja
  properties:
    zone: us-east1-c
    sizeGb: 20
    diskType: pd-standard
- name: mysql
  type: persistentDisk.jinja
  properties:
    zone: us-east1-c
    sizeGb: 20
    diskType: pd-standard
```

The first three lines import the template files we defined above and allows them to be used later on in the configuration. The `resources:` list is where all the magic happens. There are three resources declared in the list.

The first one defines the GKE cluster to be provisioned. Instead of declaring the `type` to be `container.v1.cluster` we set it to the name of our cluster template (`cluster.jinja`). The `properties:` section is where we supply the values that the template uses (remember the `{{ properties['propertyName'] }}` syntax). This cluster will be comprised of two nodes. In GKE, nodes are just [Compute VM Instances](https://cloud.google.com/compute/docs/instances/), so I configure the cluster to use `g1-small` [instance sizes](https://cloud.google.com/compute/docs/machine-types#predefined_machine_types). Additionally, I also specify that their boot disks should only be 50GB each. For the `username` and `password` you should use something that's not easy to guess. These credentials will allow anyone to access the Kubernetes APIs that are exposed by the cluster.

Now that we have our deployment defined, we can actually run it:

```bash
$ gcloud deployment-manager deployments create blog --configuration cluster.yaml
```

This operation make take a few minutes to complete as it provisions the cluster and disks. Once it completes, you should be able to see the new deployment in the Deployment Manager dashboard for your project.

To be able to use the cluster from `kubectl` we will need to run the following commands from the terminal:

```bash
$ gcloud config set container/cluster blog-blog-cluster
$ gcloud container clusters get-credentials blog-blog-cluster
```

The first command sets `blog-blog-cluster` as the default cluster for `gcloud container` commands. The second command preloads `kubectl` with credentials for that cluster.

## MySQL Deployment

Now that the GKE cluster has been created we can start deploying components to it. Since Ghost requires a database in order to run, the MySQL components will need to be up and running beforehand.

###### mysql-deployment.yaml

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: mysql-deployment
  labels:
    name: mysql
    role: mysql-deployment
spec:
  replicas: 1
  template:
    metadata:
      name: mysql-container
      labels:
        app: mysql-container
    spec:
      containers:
        - image: mysql:5.6
          name: mysql-container
          env:
            - name: MYSQL_ROOT_PASSWORD
              value: <mysql root password>
            - name: MYSQL_USER
              value: <mysql user name>
            - name: MYSQL_PASSWORD
              value: <mysql user password>
            - name: MYSQL_DATABASE
              value: ghostblog
          ports:
            - containerPort: 3306
              name: mysql-port
          volumeMounts:
            - name: mysql-persistent-storage
              mountPath: /var/lib/mysql
      volumes:
        - name: mysql-persistent-storage
          gcePersistentDisk:
            pdName: blog-disk-mysql
            fsType: ext4
```

This describes the Kubernetes Deployment that will be managing the MySQL Pod in the cluster. Since Deployments are still a beta feature, `extensions/v1beta1` needs to be set as the `apiVersion`. This allows us to use a `kind` of `Deployment`.

In the `metadata` section we define the name of the deployment as `mysql-deployment`. The `labels` are a place where you can put arbitrary metadata describing the kubernetes component. For instance you could add a value describing what environment the component is in or, what part of the system it belongs to (frontend, backend, etc.).

The root level `spec` section is where we define the nuts and bolts of the Deployment. The [spec section](http://kubernetes.io/docs/user-guide/deployments/#writing-a-deployment-spec) is where we can define how many Pod replicas should be running, which update strategy to use, and define the template for the Pod. The first part of the spec says that we would like it to create and manage one replica of the MySQL Pod. This means when you create the Deployment on the cluster for the first time, it will automatically spin up one instance of the Pod. Additionally, if the Pod crashes or becomes unhealthy the Deployment will automatically launch a new Pod instance and destroy the old one.

The `template` section is where we describe how the Pod should be created. The `metadata` section works the same way as it does for the Deployment, except all of the properties/values defined it in apply to the Pod itself. The Pod `spec` section is where the containers and volumes are defined. The `containers` section is where you define what Docker images Kubernetes needs to pull and what environment variables are set on them. There's quite a bit going on with this so let's break it down piece by piece.

```yaml
- image: mysql:5.6
  name: mysql-container
```

This tells Kubernetes to pull the MySQL 5.6 Docker image that is hosted on [Docker Hub](https://hub.docker.com/_/mysql/). We then name this container `mysql-container`.

```yaml
env:
- name: MYSQL_ROOT_PASSWORD
  value: <mysql root password>
- name: MYSQL_USER
  value: <mysql user name>
- name: MYSQL_PASSWORD
  value: <mysql user password>
- name: MYSQL_DATABASE
  value: ghostblog
```

The `env` section is where we can define environment variables that are set when the Docker container is run. The MySQL Docker container entry point script is coded to check for certain environment variables, which allow you to customize the MySQL set up.

1. `MYSQL_ROOT_PASSWORD` - The password which is set for the root MySQL account
2. `MYSQL_USER` - The name for a non-admin user account that is generated.
3. `MYSQL_PASSWORD` - The password which is used for the newly created user account.
4. `MYSQL_DATABASE` - A database which is automatically generated. The user account described above is automatically granted permissions to use this database.

The MySQL user and password is what the Ghost instance will be using to access the MySQL instance. The MySQL database (`ghostblog`) is what Ghost will be using to persist its data.

```yaml
ports:
- containerPort: 3306
  name: mysql-port
```

The ports section is where we can define what ports this Pod exposes. By default, the MySQL Docker image exposes port 3306. That's the port we will also need to expose from the Pod.

```yaml
volumeMounts:
- name: mysql-persistent-storage
  mountPath: /var/lib/mysql
```

Here we define a [Docker Volume](https://docs.docker.com/engine/userguide/containers/dockervolumes/#manage-data-in-containers) mount point. The name of the mount point (`mysql-persistent-storage`) is the name of the volume that we declare in the `volumes` section below. The `mountPath` is the folder where the volume will be mounted in the container environment. In this case, `/var/lib/mysql` is the folder that MySQL will use to write its database files.

```yaml
volumes:
- name: mysql-persistent-storage
  gcePersistentDisk:
    pdName: blog-disk-mysql
    fsType: ext4
```

Here is where we declare the volume which is mounted in the MySQL Docker container. As mentioned earlier, the name of the volume is `mysql-persistent-storage`. Kubernetes has built-in support for a wide variety of [volume types](http://kubernetes.io/docs/user-guide/volumes/#types-of-volumes) including support for mounting Google Cloud Persistent Disks. To do this we use the [gcePersistentDisk](http://kubernetes.io/docs/user-guide/volumes/#gcepersistentdisk) volume type. This volume type requires that we tell it which persistent disk to mount and what filesystem type it is. `pdName` is what we use to tell it what persistent disk to mount. In this case it is the `blog-disk-mysql` disk that got provisioned earlier. `fsType` is where we declare what type of filesystem to use for the disk. Since MySQL is running on a Linux system we can use the `ext4` filesystem type.

With the Deployment defined, we can create it on the cluster:

```bash
$ kubectl create -f mysql-deployment.yaml
```

This command uploads the deployment spec to the GKE cluster. Once the deployment has been created, it will automatically create one MySQL Pod instance (as per the spec):

```bash
$ kubectl get pods
```

This command will output a list of all the pods running in the GKE cluster. There should be a Pod name starting with `mysql-deployment-<number>-<short string>`, which is the pod that the deployment created. It might take a minute or two for this Pods status to switch from `CREATING` to `STARTED`.

Once the Pod is running, we will need a way of making it accessible to other Pods running in the cluster. To do this we will create a Kubernetes [Service](http://kubernetes.io/docs/user-guide/services/) which will allow the Ghost Pod to locate the MySQL instance.

###### mysql-service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
   labels:
      name: mysql
   name: mysql
spec:
   ports:
      - port: 3306
   selector:
      app: mysql-container
```

If you recall, port 3306 is the port that Pod exposes and so it is the port that this service maps to. The selector works by matching `labels` in the `metadata` section of Pods. All instances of the MySQL pod have a custom label of `app: mysql-container` so we use that as our selector condition.

Now we can start the service on the cluster:

```bash
$ kubectl create -f mysql-service.yaml
```

## Ghost Deployment

Now that we have MySQL running it is time to get Ghost up and running. Before we can create the kubernetes components, we are going to have to create a custom Docker image for Ghost. We need to do this so that we can supply a [custom configuration file](http://support.ghost.org/config/) to Ghost, which tells it how to connect to the MySQL database.

### Custom Ghost Image

I will outline the steps I used for setting up a custom Ghost Docker image which allows me to seed in a custom configuration file. Lets start with the Dockerfile:

###### Dockerfile

```dockerfile
FROM ghost:0.8.0
MAINTAINER Thomas Mullaly <thomas.mullaly@gmail.com>

COPY ./config.js $GHOST_SOURCE/config.example.js
COPY ./entry-override.sh /entry-override.sh

ENTRYPOINT ["/entry-override.sh"]
CMD ["npm", "start"]
```

My custom image is based off the official Docker Hub image for Ghost and is targeting the `0.8.0` release of Ghost (the most recent release of Ghost at the time of writing this blog post). The two `COPY` commands are my customization points for the image.

The first command copies my customized `config.js` file into the `$GHOST_SOURCE` of the Docker image, with a name of `config.example.js`. `$GHOST_SOURCE` is an environment variable, defined by the base Ghost Docker image, which points to the directory where the Ghost installation files are downloaded to. The `config.example.js` file will be copied by the start up script to the persistent disk when the container launches.

The second `COPY` command moves a small helper bash script into the root of the Docker container. When this script runs it will delete the existing `config.js` from the persistent disk and then runs the original Ghost docker [entry point script](https://github.com/docker-library/ghost/blob/master/docker-entrypoint.sh). We have to delete the existing `config.js` because the original entry point script won't copy `config.example.js` once it already exists on the persistent disk. Meaning we won't be able to push config changes after the first time the container has ever launched.

The last two lines set the entry point of the image to helper script and pass `npm start` as two parameters. These parameters will be passed along to the original entry point script which intern will launch Ghost.

###### config.js

```
// snippet

production: {
  url: process.env.GHOST_URL,

  database: {
    client: 'mysql',
    connection: {
      host     : process.env.MYSQL_SERVICE_HOST,
      port     : process.env.MYSQL_SERVICE_PORT,
      user     : process.env.DB_USER,
      password : process.env.DB_PASSWORD,
      database : process.env.DB_NAME,
      charset  : 'utf8'
    },
    debug: false
  },

  server: {
    host: '0.0.0.0',
    port: '2368'
  },

  paths: {
    contentPath: path.join(process.env.GHOST_CONTENT, "/")
  }
}
```

I am only showing the production configuration as it is the one that will be used when running this Docker image in your GKE cluster. A very important piece to note is in the `database.connection` section. The `host` and `port` properties are set environment variables starting with `MYSQL_SERVICE_`. At runtime, these variables will be pre-populated, by Kubernetes, with an internal IP address/port where the MySQL service is listening. These variables are auto-generated by Kubernetes and are one of a few ways that Kubernetes facilitates [service discovery](http://kubernetes.io/docs/user-guide/services/#discovering-services) inside of the cluster. The other environment variables will be defined by the Ghost Pod template.

Next step is to build and push the custom image to private container registry for the project. To build the image you run:

```bash
$ docker build -t us.gcr.io/<your gcloud project id>/ghost:v1 .
```

This will build a Docker image for your private container registry (`us.gcr.io/...`) and will tag it with `v1`. You can read more about using the private container registry [here](https://cloud.google.com/container-registry/docs/pushing). Next step is to push the image to the private registry:

```bash
$ gcloud docker push us.gcr.io/<your gcloud project id>/ghost:v1
```

The reason we use `gcloud` to push the Docker image is because `gcloud` will automatically supply the correct account credentials to Docker so that it can push the image to your private registry.

### Kubernetes

Now that we have our custom Ghost docker image pushed to our private registry, we can begin deploying the Ghost components to the cluster.

###### ghost-deployment.yaml

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
   name: ghost-deployment
spec:
   replicas: 1
   template:
      metadata:
         name: ghost-container
         labels:
            app: ghost-container
      spec:
         containers:
            - image: us.gcr.io/<project id>/ghost:v1
              name: ghost-container
              env:
                 - name: NODE_ENV
                   value: "production"
                 - name: GHOST_URL
                   value: <url to your blog>
                 - name: GHOST_CONTENT
                   value: /var/lib/ghost
                 - name: DB_USER
                   value: <your mysql user account>
                 - name: DB_PASSWORD
                   value: <your mysql user password>
                 - name: DB_NAME
                   value: <your database name>
              ports:
                 - containerPort: 2368
                   name: ghost-http
              volumeMounts:
                 - name: ghost-persistent-storage
                   mountPath: /var/lib/ghost
         volumes:
            - name: ghost-persistent-storage
              gcePersistentDisk:
                 pdName: blog-disk-ghost
                 fsType: ext4
```

This deployment is similar to the MySQL one described earlier, so I will only focus on the Ghost specific parts. We tell it to pull our custom Docker image from our private container registry. You do not need to worry about preloading credentials for the private registry into the cluster since GKE will handle this for you.

The `NODE_ENV` environment variable is what Ghost uses to determine which configuration entry it should use in its `config.js` file. Since we specify `production` it will use the production config section. The rest of the environment variables match up with the environment variables that the customized `config.js` is expecting. `GHOST_CONTENT` is a filesystem path where Ghost will store its content files (themes, post images, etc.). This path is where the Ghost persistent disk will be mounted. The `DB_*` environment variables will need to match what you entered for the MySQL Pod.

The Ghost docker image runs on port 2368, so we expose that port from the Pod. The `volumeMounts` and `volumes` are similar to the MySQL deployment. Except instead of using the persistent disk for MySQL it uses the persistent disk for the Ghost.

Creating the deployment on the cluster works the same way as the MySQL Deployment:

```bash
$ kubectl create -f ghost-deployment.yaml
```

Now we can set up the Service which will give the Ghost instance a publicly accessible IP address.

###### ghost-service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    name: ghost-service
  name: ghost-service
spec:
  type: LoadBalancer
  ports:
    - port: 80
      targetPort: 2368
      protocol: TCP
  selector:
    app: ghost-container
```

Unlike the MySQL service, we specify a type of [`LoadBalancer`](http://kubernetes.io/docs/user-guide/services/#type-loadbalancer) for this Service. Setting the type to `LoadBalancer` will cause GKE to generate a publicly accessible IP address for the service. The one port defined for this service allows it to be accessible on port 80 (the default HTTP port). When traffic comes into the Service on port 80, it will then route the requests to the specified `targetPort` of 2368 (the port Ghost runs on). This traffic is routed to any Pod that matches the selector condition of `app: ghost-container` (our Pod running the Ghost Docker container.)

To create the service on the cluster run:

```bash
$ kubectl create -f ghost-service.yaml
```

Once the service is created, it will work on provisioning a public IP address for the service. To check if the IP address is available you can run the following command:

```bash
$ kubectl get svc ghost-service
NAME            CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
ghost-service   10.7.252.127                 80/TCP     6s
```

If the `EXTERNAL-IP` is empty then the GKE cluster is still working to generate a public IP address. It might take a few minutes for this operation to complete. Once it is finished and external IP address will be listed:

```bash
$ kubectl get svc ghost-service
NAME            CLUSTER-IP     EXTERNAL-IP       PORT(S)    AGE
ghost-service   10.7.252.127   104.196.126.155   80/TCP     1m
```

Using the external IP we can view Ghost using a browser.

## Clean Up

If you are planning on using this blog you can skip these steps. To avoid charges for this set up, we will need to delete everything that was created. It is **very important** to follow these directions in order to avoid being charged for anything. The first step is to remove all of the Kubernetes components:

```bash
$ kubectl delete -f ghost-deployment.yaml
$ kubectl delete -f ghost-service.yaml
$ kubectl delete -f mysql-deployment.yaml
$ kubectl delete -f mysql-service.yaml
```

This will tear down the Pods that were created and delete the load balancer that was created for the Ghost service. Next steps are to delete the cluster and persistent disks.

Luckily, since we used Deployment Manager, cleaning up the resources can be done by deleting the deployment created at the beginning of the post:

```bash
$ gcloud deployment-manager deployments delete blog
```

The last pieces to remove are the Docker images in the private Container Registry. There is not really a clear cut solution for deleting these images from the registry. The only suggestions I have [found so far](http://stackoverflow.com/a/33791574) is to just delete the underlying Storage bucket that is used to house the images. This can be done pretty easily using the `gsutil` command line utility that is installed along side `gcloud`.

```bash
$ gsutil rb gs://us.artifacts.<project id>.appspot.com
```

## Next Steps

Due to the amount of information that needed to be covered, I paired this Ghost setup down to a minimum. Future blog posts will document how to store credentials using Kubernetes Secrets, configure SSL using an Nginx reverse proxy, and deploying custom themes with the Ghost container. Stay tuned for future blog posts which document these steps.

If you would like to get this up and running, I have hosted all of the code on GitHub. You can check it out [here](https://github.com/thomas-mullaly/ghost-mysql-gke).