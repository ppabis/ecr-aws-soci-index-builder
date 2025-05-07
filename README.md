AWS ECR SOCI index generator
=======================

This tool is based on [cfn-ecr-aws-soci-index-builder](https://github.com/awslabs/cfn-ecr-aws-soci-index-builder)
which was originally a Cloudformation that deployed Lambda function to generate
a SOCI index for each image pushed to ECR. It worked using EventBridge that
emitted an event on an image push to ECR that triggered a Lambda function that
downloaded the image locally using ORAS, keeping it as files, and parts of
another AWS project [soci-snapshotter](https://github.com/awslabs/soci-snapshotter)
were used to generate the index.

Compiling
---------

Just go to `soci-index-generator-standalone` and build the provided
`Dockerfile`.

```bash
cd soci-index-generator-standalone
docker build -t ppabis/soci-index-generator-standalone:latest .
```

Usage
-----

There are two flags - required `-repository` where you give the full URI to the
image and `-min-layer-size` which controls what is the smallest layer to index.
Default `min-layer-size` is 10 megabytes.

For credentials you should use environment variables (or mounting the
credentials file). You also need to provide a region to use. For example if you
have an assumed role you can use the following command.

```bash
docker run --rm -it \
 -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID \
 -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY \
 -e AWS_SESSION_TOKEN=$AWS_SESSION_TOKEN \
 -e AWS_REGION=eu-west-1 \
 ppabis/soci-index-generator-standalone:latest \
 -repository 123456789012.dkr.ecr.eu-west-1.amazonaws.com/test-repository:latest \
 -min-layer-size 1000720
```
