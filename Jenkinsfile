pipeline {
    agent any
    environment {
        GETH_EXPORTS = '/data/ethereum-exports'
        GETH_DATADIR = '/data/geth'
        GITHUB_NOTIFY_DESCRIPTION = 'Assert import of canonical chain data'
    }
    stages {
        stage('Notify Github of Pending Jobs') {
            steps {
                githubNotify context: 'Kotti Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'PENDING', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}"
                githubNotify context: 'Mordor Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'PENDING', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}"
                githubNotify context: 'Goerli Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'PENDING', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}"
                // githubNotify context: 'Classic Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'PENDING', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}"
                // githubNotify context: 'Foundation Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'PENDING', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}"
            }
        }
        stage("Run Regression Tests") {
            parallel {
                stage('Kotti') {
                    agent { label "aws-slave-m5-xlarge" }
                    steps {
                        sh "make geth && ./build/bin/geth version"
                        sh "rm -rf ${GETH_DATADIR}-kotti"
                        sh "sha256sum --check <(echo 'd000df15854efa14fccc899913bff7c6b482fd1a5781d1b12c60ada5ae99d281 ${GETH_EXPORTS}/kotti.0-2544960.rlp.gz')"
                        sh "./build/bin/geth --kotti --cache=2048 --nocompaction --nousb --txlookuplimit=1 --datadir=${GETH_DATADIR}-kotti import ${GETH_EXPORTS}/kotti.0-2544960.rlp.gz"
                    }
                    post {
                        always { sh "rm -rf ${GETH_DATADIR}-kotti" }
                        success { githubNotify context: 'Kotti Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'SUCCESS', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                        unsuccessful { githubNotify context: 'Kotti Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'FAILURE', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                    }
                }
                stage('Mordor') {
                    agent { label "aws-slave-m5-xlarge" }
                    steps {
                        sh "make geth && ./build/bin/geth version"
                        sh "rm -rf ${GETH_DATADIR}-mordor"
                        sh "sha256sum --check <(echo 'b101c6180c32457fd29ce3a5f58d4a08ff6a1af21a0ac3e4a5b5e4502531e066 ${GETH_EXPORTS}/mordor.0-1686858.rlp.gz')"
                        sh "./build/bin/geth --mordor --fakepow --cache=2048 --nocompaction --nousb --txlookuplimit=1 --datadir=${GETH_DATADIR}-mordor import ${GETH_EXPORTS}/mordor.0-1686858.rlp.gz"
                        sh "rm -rf ${GETH_DATADIR}"
                    }
                    post {
                        always { sh "rm -rf ${GETH_DATADIR}-mordor" }
                        success { githubNotify context: 'Mordor Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'SUCCESS', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                        unsuccessful { githubNotify context: 'Mordor Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'FAILURE', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                    }
                }
                stage('Goerli') {
                    agent { label "aws-slave-m5-xlarge" }
                    steps {
                        sh "make geth && ./build/bin/geth version"
                        sh "rm -rf ${GETH_DATADIR}-goerli"
                        sh "sha256sum --check <(echo 'de816468326e9ef16ee509b903948df3045cc2d2ee470a61fca9757042db9f35 ${GETH_EXPORTS}/goerli.0-2000000.rlp.gz')"
                        sh "./build/bin/geth --goerli --cache=2048 --nocompaction --nousb --txlookuplimit=1 --datadir=${GETH_DATADIR}-goerli import ${GETH_EXPORTS}/goerli.0-2000000.rlp.gz"
                    }
                    post {
                        always { sh "rm -rf ${GETH_DATADIR}-goerli" }
                        success { githubNotify context: 'Goerli Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'SUCCESS', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                        unsuccessful { githubNotify context: 'Goerli Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'FAILURE', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                    }
                }
                // Commented now because these take a looong time.
                // One way of approaching a solution is to break each chain into a "stepladder" of imports, eg. 0-1150000, 1150000-1920000, 1920000-2500000, etc...
                // This would allow further parallelization at the cost of duplicated base chaindata stores.
                // Since the core focus of testing here is configuration (both user-facing and internal), and one of ugly limitations of our current testnets 
                // being that they DO NOT reflect the production environment well in this regard (which is a very vulnerable reagard) 
                // another approach might be to condense the chain fork progressions of ETC and ETH into custom test-only chains, perhaps using retestest or a similar
                // tool to make transactions and manage chain upgrades dynamically as a transactions are made.
                // 
                // stage('Classic') {
                //     agent { label "aws-slave-m5-xlarge" }
                //     steps {
                //         sh "make geth && ./build/bin/geth version"
                //         sh "rm -rf ${GETH_DATADIR}-classic"
                //         sh "./build/bin/geth --classic --cache=2048 --nocompaction --nousb --txlookuplimit=1 --datadir=${GETH_DATADIR}-classic import ${GETH_EXPORTS}/classic.0-10620587.rlp.gz"
                //     }
                //     post {
                //         always { sh "rm -rf ${GETH_DATADIR}-classic" }
                //         success { githubNotify context: 'Classic Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'SUCCESS', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                //         unsuccessful { githubNotify context: 'Classic Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'FAILURE', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                //     }
                // }
                // stage('Foundation') {
                //     agent { label "aws-slave-m5-xlarge" }
                //     steps {
                //         sh "make geth && ./build/bin/geth version"
                //         sh "rm -rf ${GETH_DATADIR}-foundation"
                //         sh "./build/bin/geth --cache=2048 --nocompaction --nousb --txlookuplimit=1 --datadir=${GETH_DATADIR}-foundation import ${GETH_EXPORTS}/ETH.0-10229163.rlp.gz"
                //     }
                //     post {
                //         always { sh "rm -rf ${GETH_DATADIR}-foundation" }
                //         success { githubNotify context: 'Foundation Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'SUCCESS', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                //         unsuccessful { githubNotify context: 'Foundation Regression', description: "${GITHUB_NOTIFY_DESCRIPTION}", status: 'FAILURE', account: 'etclabscore', repo: 'core-geth', credentialsId: 'meowsbits-github-jenkins', sha: "${GIT_COMMIT}" }
                //     }
                // }
            }
        }
    }
}
