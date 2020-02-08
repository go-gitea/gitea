# git hook scripts
The subfolders contain common git hook scripts that were updated to be used directly with Gitea. If you are looking for scripts, you can have a look at [Github platform samples](https://github.com/github/platform-samples). If you have own scripts which are from general interest, feel free to share them here.

### local/user scripts
Those scripts can either be used as local scripts the user can add to his repository (if not disabled in Gitea configuration file).

### global scripts
Those scripts can also appied to all newly created repositories by git automatically, when following these steps below.

Note:<br />
Gitea runs git commands as user `RUN_USER` defined in Gitea configuration file. 
In the following steps, it is assumed a home directory was created for that user and the global hook scripts shall be placed there. It is also possible to use a different path that is fully accessible by the `RUN_USER`. In order to execute the following shell commands, replace `<RUN_USER>` with the corresponding user name.

1. Create the folders to store the global git hook scripts [needs only to be done once]:
    ```
    sudo -S -u <RUN_USER> mkdir -p ~/.git-templates/hooks/{pre-receive.d,post-receive.d,update.d}
    ```

1. Enable the global git templates and with it the hook scripts [needs only to be done once]:
    ```
    sudo -S -u <RUN_USER> git config --global init.templatedir '~/.git-templates'
    ```

1. Add the desired global hook scripts to the correct folders (this example uses `commit-current-user-check.sh`):
    ```
    sudo -S -u <RUN_USER> wget -O ~/.git-templates/hooks/pre-receive.d/commit-current-user-check.sh https://github.com/go-gitea/gitea/raw/master/contrib/git/git-hook-scripts/pre-receive.d/commit-current-user-check.sh
    ```

1. Make the script executable:
    ```
    sudo -S -u <RUN_USER> chmod 740 ~/.git-templates/hooks/pre-receive.d/commit-current-user-check.sh
    ```
