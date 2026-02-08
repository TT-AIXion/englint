class Englint < Formula
  desc "CLI linter for detecting non-English text in source files"
  homepage "https://github.com/TT-AIXion/englint"
  url "https://github.com/TT-AIXion/englint/archive/refs/tags/v0.0.0.tar.gz"
  sha256 "REPLACE_WITH_RELEASE_SHA256"
  license "MIT"

  def install
    bin.install "englint"
    man1.install "docs/englint.1"
    bash_completion.install "completions/englint.bash" => "englint"
    zsh_completion.install "completions/englint.zsh" => "_englint"
  end

  test do
    system "#{bin}/englint", "version"
  end
end
