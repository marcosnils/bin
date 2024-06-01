FROM mcr.microsoft.com/devcontainers/go:1-1.22-bookworm
LABEL version="1.0"
LABEL maintainer="perplexedpigmy@xmousse.com"
LABEL description="Go+"

ARG USERNAME=vscode

SHELL ["/bin/bash", "-c"]

RUN \
  # 
  # Persist history between different builds
  SNIPPET="PROMPT_COMMAND='history -a' && export HISTFILE=/cmdhistory/.bash_history"  \
  && mkdir /cmdhistory  \
  && touch /cmdhistory/.bash_history  \
  && chown -R $USERNAME /cmdhistory/.bash_history  \
  && echo $SNIPPET >> /home/$USERNAME/.bashrc   \
  #
  # Cleanups 
  && apt update -y \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* 