# buildkansen

Fast, pooled, custom macOS runners for GitHub Actions. Just change a single line in your workflows to get faster and cheaper iOS/macOS builds.

## Deployment

The ansible script to deploy/setup mac machines relies on SSH agent forwarding, ensure you something like this configured in `~/.ssh/config`:

```bash
Host <host>
  ForwardAgent yes
```

And also ensure your SSH key (that has access to the repo) is added to the agent:

```bash
ssh-add -L
ssh-add -K ~/.ssh/id_rsa_github
ssh-add -L
```