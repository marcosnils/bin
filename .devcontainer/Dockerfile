FROM mcr.microsoft.com/devcontainers/go:1-1.22-bookworm
LABEL version="1.0"
LABEL maintainer="perplexedpigmy@xmousse.com"
LABEL description="Go++"

ARG USERNAME=vscode
ARG BIN_EXE_DIR=/home/${USERNAME}/.local/bin

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
  && sudo -i -u $USERNAME mkdir -p ${BIN_EXE_DIR} \
  # Insatll the bin utilit
  && sudo -i -u $USERNAME wget -O /tmp/bin https://github.com/perplexedpigmy/bin/releases/download/v0.17.5/bin_0.17.5_linux_amd64 \
  && sudo -i -u $USERNAME chmod a+x /tmp/bin \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} /tmp/bin install github.com/perplexedpigmy/bin" \
  && sudo -i -u $USERNAME rm /tmp/bin \
  # 
  # I
  && find ${BIN_EXE_DIR} \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} ${BIN_EXE_DIR}/bin install github.com/Wilfred/difftastic" \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} ${BIN_EXE_DIR}/bin install github.com/casey/just -s :just" \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} ${BIN_EXE_DIR}/bin install github.com/ogham/exa -s exa-linux-x86_64-v0.10.1.zip:bin/exa" \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} ${BIN_EXE_DIR}/bin install github.com/sharkdp/bat -s bat-v0.24.0-x86_64-unknown-linux-gnu.tar.gz:bat-v0.24.0-x86_64-unknown-linux-gnu/bat" \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} ${BIN_EXE_DIR}/bin install github.com/mikefarah/yq  -s yq_linux_amd64" \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} ${BIN_EXE_DIR}/bin install github.com/direnv/direnv"  \
  && sudo -i -u $USERNAME bash -c "BIN_EXE_DIR=${BIN_EXE_DIR} ${BIN_EXE_DIR}/bin install github.com/BurntSushi/ripgrep -s ripgrep-14.1.0-x86_64-unknown-linux-musl.tar.gz:ripgrep-14.1.0-x86_64-unknown-linux-musl/rg" \
  
  # Cleanups 
  && apt update -y \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* 