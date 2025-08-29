pipeline {
    agent any
    
    environment {
        GO_VERSION = '1.21'
        DOCKER_IMAGE = 'gatekeeper'
        DOCKER_TAG = "${BUILD_NUMBER}"
    }
    
    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }
        
        stage('Setup Go') {
            steps {
                script {
                    def goTool = tool name: "go-${GO_VERSION}", type: 'go'
                    env.PATH = "${goTool}/bin:${env.PATH}"
                }
            }
        }
        
        stage('Install Dependencies') {
            steps {
                sh 'go mod download'
                sh 'go mod verify'
            }
        }
        
        stage('Lint') {
            steps {
                sh 'which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2'
                sh 'golangci-lint run'
            }
        }
        
        stage('Test') {
            steps {
                sh 'go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...'
            }
            post {
                always {
                    publishCoverage adapters: [
                        coberturaAdapter('coverage.xml')
                    ], sourceFileResolver: sourceFiles('STORE_LAST_BUILD')
                }
            }
        }
        
        stage('Security Scan') {
            steps {
                sh 'which gosec || go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest'
                sh 'gosec ./...'
            }
        }
        
        stage('Build') {
            steps {
                sh 'CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gatekeeper .'
            }
        }
        
        stage('Docker Build') {
            steps {
                script {
                    def image = docker.build("${DOCKER_IMAGE}:${DOCKER_TAG}")
                    docker.withRegistry('https://registry.hub.docker.com', 'docker-hub-credentials') {
                        image.push()
                        image.push('latest')
                    }
                }
            }
        }
        
        stage('Deploy to Staging') {
            when {
                branch 'develop'
            }
            steps {
                script {
                    // Deploy to staging environment
                    sh '''
                    echo "Deploying to staging..."
                    # Add your staging deployment commands here
                    '''
                }
            }
        }
        
        stage('Deploy to Production') {
            when {
                branch 'main'
            }
            steps {
                script {
                    // Deploy to production environment
                    sh '''
                    echo "Deploying to production..."
                    # Add your production deployment commands here
                    '''
                }
            }
        }
    }
    
    post {
        always {
            cleanWs()
        }
        success {
            slackSend(
                color: 'good',
                message: "✅ Pipeline succeeded for ${env.JOB_NAME} - ${env.BUILD_NUMBER}"
            )
        }
        failure {
            slackSend(
                color: 'danger', 
                message: "❌ Pipeline failed for ${env.JOB_NAME} - ${env.BUILD_NUMBER}"
            )
        }
    }
}