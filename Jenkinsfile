pipeline {
    agent { label 'ubuntu' }
    tools {
      go '1.19.6'
      nodejs '18.10.0'
    }
        stages {
        stage('Cloning git reposiry') {
            steps {
                git branch: 'release/v1.17', url: 'git@github.com:bortnychuk/gitea.git'
            }
        }
        stage('Configuring Go, nodejs and build-essential') {
            steps {
                sh 'npm version'
                sh 'go version'
                sh 'sudo apt install make'
                sh 'sudo apt-get install build-essential'
                sh 'sudo go install github.com/google/go-licenses@latest'
                sh 'ls -l'
            }
        }
        stage('Building Gitea application') {
            steps {
                sh 'TAGS="bindata sqlite sqlite_unlock_notify" make build'
            }
        }
        stage('Running tests') {
            steps {
                sh 'TAGS="bindata sqlite sqlite_unlock_notify" make test'
            }
        }
        stage('Publish to Test Server') {
            steps {
                sshPublisher(publishers: [sshPublisherDesc(configName: 'MyGiteaServer', transfers: [sshTransfer(cleanRemote: false, excludes: '', execCommand: '/tmp/solo/gitea web', execTimeout: 120000, flatten: false, makeEmptyDirs: false, noDefaultExcludes: false, patternSeparator: '[, ]+', remoteDirectory: '/tmp/solo', remoteDirectorySDF: false, removePrefix: '', sourceFiles: '*')], usePromotionTimestamp: false, useWorkspaceInPromotion: false, verbose: false)])
            }
        }
    }        
}
