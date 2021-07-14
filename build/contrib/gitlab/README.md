# GitLab CI/CD configuration for Mattermost

The [.gitlab-ci.yml](./.gitlab-ci.yml) file in this directory provides a [GitLab CI/CD configuration file](https://docs.gitlab.com/ee/ci/yaml/gitlab_ci_yaml.html) that can be used to run linting and unit testing for the Mattermost application.

## Usage

To configure your GitLab instance to use this configuration file (and without having to move it to the base directory in this git repository), you can configure the [`CI_CONFIG_PATH` predefined variable](https://docs.gitlab.com/ee/ci/variables/predefined_variables.html) for the repository in GitLab with the configuration file path (`build/contrib/gitlab/.gitlab-ci.yml`).
