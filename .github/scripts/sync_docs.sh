#!/usr/bin/env bash

# Some env variables
BRANCH="master"
MAJOR_VERSION="v2"
REPO_URL="github.com/gofiber/docs.git"
AUTHOR_EMAIL="github-actions[bot]@users.noreply.github.com"
AUTHOR_USERNAME="github-actions[bot]"

# Set commit author
git config --global user.email "${AUTHOR_EMAIL}"
git config --global user.name "${AUTHOR_USERNAME}"

if [[ $EVENT == "push" ]]; then
    latest_commit=$(git rev-parse --short HEAD)
    log_output=$(git log --oneline ${BRANCH} HEAD~1..HEAD --name-status -- docs/)

    if [[ $log_output != "" ]]; then
        git clone https://${TOKEN}@${REPO_URL} fiber-docs
        cp -a docs/* fiber-docs/docs
        
        # Push changes for next docs
        cd fiber-docs/ || return
        git add .
        git commit -m "Add docs from https://github.com/gofiber/fiber/commit/${latest_commit}"
        git push https://${TOKEN}@${REPO_URL}
    fi
elif [[ $EVENT == "release" ]]; then
    latest_tag=$(git describe --tags --abbrev=0)

    # Push changes for stable docs
    git clone https://${TOKEN}@${REPO_URL} fiber-docs
    cd fiber-docs/ || return
    cp -a docs/* versioned_docs/version-${MAJOR_VERSION}.x
    git add .
    git commit -m "Sync docs for ${latest_tag} release"
    git push https://${TOKEN}@${REPO_URL}
fi