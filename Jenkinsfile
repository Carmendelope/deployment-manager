def slack = new org.daisho.Slack()

def packageName = "deployment-manager"
def appsList = "deployment-manager deployment-manager-cli"
def packagePath = "src/github.com/nalej/${packageName}"

pipeline {
    agent { node { label 'golang' } }
    options {
        checkoutToSubdirectory("${packagePath}")
        buildDiscarder(logRotator(numToKeepStr: '10'))
    }

    stages {
        stage("Variable initialization") {
            steps { stepVariableInitialization packagePath }
        }
        stage("Git setup") {
            steps { container("golang") { stepGitSetup() } }
        }
        stage("Dependency download") {
            steps { container("golang") { stepGolangDependencyDownload packagePath } }
        }
        stage("Unit tests") {
            steps { container("golang") { stepGolangUnitTests packagePath } }
        }
        stage("Binary compilation") {
            steps { container("golang") { stepGolangBinaryCompilation packagePath, appsList } }
        }
        stage("Publish image to Docker") {
            when { branch 'master' }
            steps { container("docker") { stepPublishToDocker packagePath, appsList, "nalej" } }
        }
    }

    post {
        success { script { slack.sendBuildNotification("success", "good") } }
        failure { script { slack.sendBuildNotification("failure", "danger") } }
        aborted { script { slack.sendBuildNotification("aborted", "warning") } }
    }
}
