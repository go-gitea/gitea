import React from "react"

import { emojify } from "react-emoji"
import { SmileyIcon } from "@primer/octicons-react"

const {csrf} = window.config;

class AddReaction extends React.Component {
  state = { choices: [] }

  constructor(p) {
    super(p)

    fetch("/api/v1/settings/ui", { headers: {"accept": "application/json"}})
    .then(response => response.json())
    .then(response => {
      this.setState({ choices: response.allowed_reactions })
    })
  }

  render = () => (
    <div
      className="item action ui pointing select-reaction dropdown top right"
      data-action-url={this.props.address}
    >
      <a className="add-reaction" >
        <SmileyIcon />
      </a>

      <div className="menu">
        <div className="header">
          {this.props.phrases.pick}
        </div>

        <div className="divider"></div>

        {this.state.choices.map(r => (
          <div key={r} className="item reaction" data-content={r} >
            {emojify(`:${r}:`, { emojiType: 'emojione' })}
          </div>
        ))}
      </div>
    </div>
  )

  componentDidMount() {
    initReactionSelector(null, this.props.rerender)
  }

  /*
    choices={response.allowed_reactions}
    actionURL={p.dataset['actionUrl']}
    pick={p.dataset['i18nPick']}

        initReactionSelector($(p))
   */
}

function initReactionSelector(parent, callback) {
  let reactions = '';
  if (!parent) {
    parent = $(document);
    reactions = '.reactions > ';
  }

  parent
    .find(`${reactions}a.label`)
    .popup({
      position: 'bottom left',
      metadata: {content: 'title', title: 'none'}
    });

  parent
    .find(`.select-reaction > .menu > .item, ${reactions}a.label`)
    .on('click', function (e) {
      const vm = this;
      e.preventDefault();

      if ($(this).hasClass('disabled')) return;

      const actionURL = $(this).hasClass('item')
          ? $(this).closest('.select-reaction').data('action-url')
          : $(this).data('action-url');

      const url = `${actionURL}/${$(this).hasClass('blue')
          ? 'unreact'
          : 'react'}`;

      $.ajax({
        type: 'POST',
        url,
        data: {
          _csrf: csrf,
          content: $(this).data('content')
        }
      }).done((resp) => {

        if (resp && (resp.html || resp.empty)) {
          const content = $(vm).closest('.content');
          let react = content.find('.segment.reactions');

          if ((!resp.empty || resp.html === '') && react.length > 0) {
            react.remove();
          }

          if (!resp.empty) {
            react = $('<div class="ui attached segment reactions"></div>');
            const attachments = content.find('.segment.bottom:first');

            if (attachments.length > 0) {
              react.insertBefore(attachments);
            } else {
              react.appendTo(content);
            }

            react.html(resp.html);
            react.find('.dropdown').dropdown();

            callback(react, callback);
          }
        }
      });
    });
}

export { initReactionSelector }
export default AddReaction
