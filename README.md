# Minio resource operator

The Minio Resource Operator delivers as easy way to deliver Minio Resources declaratively via Kubernetes `CRD`
It was created in case of [issue](https://github.com/minio/operator/issues/1100) which is not supporting `Customer Resources` in [official operator](https://github.com/minio/operator)

### Operator features

* Create service accounts

* Create policies

* Create buckets

## Installation

You need to set minio tenant configuration (endpoint and credentials) in `values.yaml`

```yaml
operator:
  env:
    - name: MINIO_ENDPOINT
      value: ''
    - name: MINIOT_ACCESS_KEY
      value: ''
    - name: MINIOT_SECRET_KEY
      value: ''
```

> In case  they are env variables, you're able to provide `valueFrom` and ref to secret

## Resources

Deploy Custom Resource to manage resource under minio

> Minio should be already provisioned and operator as well

### Policy
```yaml
apiVersion: minio-resource-operator.pannoi/v1beta1
kind: Policy
metadata:
    name: policy-name
    namespace: default
spec:
    name: policy-name # PolicyName
    statement: | # AWS type policy
        {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Action": [
                        "s3:GetBucketLocation",
                        "s3:GetObject"
                    ],
                    "Resource": [
                        "arn:aws:s3:::my-bucket"
                    ]
                }
            ]
        }
```

### User
```yaml
apiVersion: minio-resource-operator.pannoi/v1beta1
kind: User
metadata:
    name: username
    namespace: default
spec:
    name: username # Username (Password would be generated automatically)
    policies:
        - policy-name # Minio policy name
```

> After user is created, operator will provision k8s `secret` automatically in provided namespace

### Bucket
```yaml
apiVersion: minio-resource-operator.pannoi/v1beta1
kind: Bucket
metadata:
    name: my-bucket
    namespace: default
spec:
    name: my-bucket
    objectLocking: 
        enabled: true 
        mode: compliance # Compliance/Governance
        retention: 180 # Retention policy configuration in days
    versioning:
        enabled: true
```