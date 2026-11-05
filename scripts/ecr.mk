ECR_REGISTRY ?= public.ecr.aws/f6j8u8s3
ECR_REPOSITORY ?= $(ECR_REGISTRY)/nurad/mediamtx-persist

ecr:
	$(eval VERSION := $(shell git describe --tags | tr -d v))
	@if [ -z "$(VERSION)" ]; then echo "ERROR: no git tag found; create one first (e.g. git tag v1.0.0)"; exit 1; fi

	# Amazon ECR Public authentication is always performed in us-east-1.
	aws ecr-public get-login-password --region us-east-1 | \
	docker login --username AWS --password-stdin $(ECR_REGISTRY)

	docker buildx rm builder 2>/dev/null || true
	docker buildx create --name=builder

	docker build --builder=builder \
	-f docker/ffmpeg-rpi.Dockerfile . \
	--platform=linux/arm/v6,linux/arm/v7,linux/arm64 \
	-t $(ECR_REPOSITORY):$(VERSION)-ffmpeg-rpi \
	-t $(ECR_REPOSITORY):1-ffmpeg-rpi \
	-t $(ECR_REPOSITORY):latest-ffmpeg-rpi \
	--push

	docker build --builder=builder \
	-f docker/rpi.Dockerfile . \
	--platform=linux/arm/v6,linux/arm/v7,linux/arm64 \
	-t $(ECR_REPOSITORY):$(VERSION)-rpi \
	-t $(ECR_REPOSITORY):1-rpi \
	-t $(ECR_REPOSITORY):latest-rpi \
	--push

	docker build --builder=builder \
	-f docker/ffmpeg.Dockerfile . \
	--platform=linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64 \
	-t $(ECR_REPOSITORY):$(VERSION)-ffmpeg \
	-t $(ECR_REPOSITORY):1-ffmpeg \
	-t $(ECR_REPOSITORY):latest-ffmpeg \
	--push

	docker build --builder=builder \
	-f docker/standard.Dockerfile . \
	--platform=linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64 \
	-t $(ECR_REPOSITORY):$(VERSION) \
	-t $(ECR_REPOSITORY):1 \
	-t $(ECR_REPOSITORY):latest \
	--push

	docker buildx rm builder
