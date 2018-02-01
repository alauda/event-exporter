# Event-exporter

Export Kubernetes' events to Elasticsearch/Kafka/HTTP Endpoint.

Inspired by https://github.com/GoogleCloudPlatform/k8s-stackdriver/tree/master/event-exporter .

# Build and run

```
$ cat Makefile
```

# How to config

## Event exporter options:
   
Common options:

```
    -prometheus-endpoint string
        Endpoint on which to expose Prometheus http handler (default ":80")
    -resync-period duration
        Reflector resync period (default 1m0s)
    -sink string
        Sink name, now suported are Elasticsearch/Kafka/HTTP endpoint.
    -sink-opts string
        Parameters for selected output sinks
    -flush-delay duration
        Delay after receiving the first event in batch before sending the request to output sink(default 5s).
    -max-buffer-size int
        Maximum number of events in the request to output sink (default 1000).
    -max-concurrency int
        Maximum number of concurrent requests to output sink (default 1).
```

## Elasticsearch


## Options for Elasticsearch

   ```
   Usage of Elasticsearch:
     -flush-delay duration
         Delay after receiving the first event in batch before sending the request to Stackdriver, if batchdoesn't get sent before (default 5s)
     -max-buffer-size int
         Maximum number of events in the request to Stackdriver (default 100)
     -max-concurrency int
         Maximum number of concurrent requests to Stackdriver (default 10)
   ```

# Deploy on kubernetes

```
apiVersion: v1
kind: Service
metadata:
  name: event-exporter
  namespace: kube-system
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    run: event-exporter
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    run: event-exporter
  name: event-exporter
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      run: event-exporter
  template:
    metadata:
      labels:
        run: event-exporter
    spec:
      containers:
      - image: liubin/event-exporter
        ports:
        - containerPort: 80 
        imagePullPolicy: Always
        name: event-exporter
        command: ["/event-exporter"]
        args: ["-v", "4"]
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      terminationGracePeriodSeconds: 30

```