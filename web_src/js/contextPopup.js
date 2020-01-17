let suburl;

window.addEventListener('load', () => {
  suburl = $('meta[name=_suburl]').attr('content');
  const refIssues = $('.ref-issue');
  if (!refIssues.length) return;

  refIssues.each(function () {
    const href = $(this).attr('href').split('/');
    const index = href.pop();
    href.pop(); // issues
    const repo = href.pop();
    const owner = href.pop();
    issuePopup(owner, repo, index, $(this));
  });
});

function issuePopup(owner, repo, index, $element) {
  $.get(`${suburl}/api/v1/repos/${owner}/${repo}/issues/${index}`, (issue) => {
    const createdAt = new Date(issue.created_at).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });

    let body = issue.body.replace(/\n+/g, ' ');
    if (body.length > 85) {
      body = `${body.substring(0, 85)}...`;
    }

    let labels = '';
    for (let i = 0; i < issue.labels.length; i++) {
      const label = issue.labels[i];
      const red = parseInt(label.color.substring(0, 2), 16);
      const green = parseInt(label.color.substring(2, 4), 16);
      const blue = parseInt(label.color.substring(4, 6), 16);
      let color = '#ffffff';
      if ((red * 0.299 + green * 0.587 + blue * 0.114) > 125) {
        color = '#000000';
      }
      labels += `<div class="ui label" style="color: ${color}; background-color:#${label.color};">${label.name}</div>`;
    }
    if (labels.length > 0) {
      labels = `<p>${labels}</p>`;
    }

    let octicon;
    if (issue.pull_request !== null) {
      if (issue.state === 'open') {
        octicon = 'green octicon-git-pull-request'; // Open PR
      } else if (issue.pull_request.merged === true) {
        octicon = 'purple octicon-git-merge'; // Merged PR
      } else {
        octicon = 'red octicon-git-pull-request'; // Closed PR
      }
    } else if (issue.state === 'open') {
      octicon = 'green octicon-issue-opened'; // Open Issue
    } else {
      octicon = 'red octicon-issue-closed'; // Closed Issue
    }

    $element.popup({
      variation: 'wide',
      delay: {
        show: 250
      },
      html: `
<div>
  <p><small>${issue.repository.full_name} on ${createdAt}</small></p>
  <p><i class="octicon ${octicon}"></i> <strong>${issue.title}</strong> #${index}</p>
  <p>${body}</p>
  ${labels}
</div>
`
    });
  });
}
