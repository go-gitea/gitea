# git hook scripts
The subfolders contain common git hook scripts that were updated to be used directly with Gitea. If you are looking for scripts, you can have a look at [Github platform samples](https://github.com/github/platform-samples). If you have own scripts which are from general interest, feel free to share them here.

### local/user scripts
Those scripts can either be used as local scripts the user can add to his repository (if not disabled in Gitea configuration file).

### global scripts
Those scripts can also appied to all newly created repositories by git automatically, when following these steps below.

Note:<br />
It is assumed that git runs as a non-priviledged user and a home directory was created for that user. In oder to execute the following shell commands, replace `<git-user>` with the real username git runs.

1. Create the folders to store the global git hook scripts [needs only to be done once]:
    ```
    sudo -S -u <git-user> mkdir -p ~/.git-templates/hooks/{pre-receive.d,post-receive.d,update.d}
    ```

1. Enable the global git templates and with it the hook scripts [needs only to be done once]:
    ```
    sudo -S -u git git config --global init.templatedir '~/.git-templates'
    ```

1. Add the desired global hook scripts to the correct folders (this example uses `commit-current-user-check.sh`):
    ```
    sudo -S -u git wget -O ~/.git-templates/hooks/pre-receive.d/commit-current-user-check.sh https://github.com/go-gitea/gitea/raw/master/contrib/git/git-hook-scripts/pre-receive.d/commit-current-user-check.sh
    ```

1. Make the script executable:
    ```
    sudo -S -u git chmod 740 ~/.git-templates/hooks/pre-receive.d/commit-current-user-check.sh
    ```
