#!/bin/bash

set -e

echo "LoadTestForge AWS Deployment Script"
echo "===================================="

AWS_REGION="${AWS_REGION:-ap-northeast-2}"
ECR_REPO_NAME="${ECR_REPO_NAME:-loadtest}"
CLUSTER_NAME="${CLUSTER_NAME:-loadtest-cluster}"
SERVICE_NAME="${SERVICE_NAME:-loadtest-service}"

AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_URL="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO_NAME}"

echo "Step 1: Building Docker image..."
docker build -t ${ECR_REPO_NAME}:latest -f deployments/docker/Dockerfile .

echo "Step 2: Logging into ECR..."
aws ecr get-login-password --region ${AWS_REGION} | \
    docker login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com

echo "Step 3: Creating ECR repository if not exists..."
aws ecr describe-repositories --repository-names ${ECR_REPO_NAME} --region ${AWS_REGION} 2>/dev/null || \
    aws ecr create-repository --repository-name ${ECR_REPO_NAME} --region ${AWS_REGION}

echo "Step 4: Tagging and pushing image..."
docker tag ${ECR_REPO_NAME}:latest ${ECR_URL}:latest
docker push ${ECR_URL}:latest

echo "Step 5: Updating task definition..."
TASK_DEF_JSON=$(cat deployments/aws/task-definition.json | \
    sed "s|YOUR_ECR_URL|${ECR_URL}|g" | \
    sed "s|YOUR_ACCOUNT_ID|${AWS_ACCOUNT_ID}|g")

TASK_DEF_ARN=$(echo "${TASK_DEF_JSON}" | \
    aws ecs register-task-definition --cli-input-json file:///dev/stdin --region ${AWS_REGION} \
    --query 'taskDefinition.taskDefinitionArn' --output text)

echo "Registered task definition: ${TASK_DEF_ARN}"

echo "Step 6: Creating CloudWatch log group if not exists..."
aws logs create-log-group --log-group-name /ecs/loadtest --region ${AWS_REGION} 2>/dev/null || true

echo "Step 7: Checking if ECS cluster exists..."
CLUSTER_EXISTS=$(aws ecs describe-clusters --clusters ${CLUSTER_NAME} --region ${AWS_REGION} \
    --query 'clusters[0].status' --output text 2>/dev/null || echo "NONE")

if [ "${CLUSTER_EXISTS}" == "NONE" ] || [ "${CLUSTER_EXISTS}" == "INACTIVE" ]; then
    echo "Creating ECS cluster..."
    aws ecs create-cluster --cluster-name ${CLUSTER_NAME} --region ${AWS_REGION}
fi

echo ""
echo "Deployment complete!"
echo ""
echo "To run a one-time task:"
echo "aws ecs run-task \\"
echo "  --cluster ${CLUSTER_NAME} \\"
echo "  --launch-type FARGATE \\"
echo "  --task-definition loadtest-task \\"
echo "  --network-configuration 'awsvpcConfiguration={subnets=[subnet-xxx],securityGroups=[sg-xxx],assignPublicIp=ENABLED}' \\"
echo "  --region ${AWS_REGION}"
echo ""
echo "To view logs:"
echo "aws logs tail /ecs/loadtest --follow --region ${AWS_REGION}"
