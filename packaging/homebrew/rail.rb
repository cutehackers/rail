class Rail < Formula
  desc "Harness control-plane for Codex"
  homepage "https://github.com/cutehackers/rail"
  url "https://github.com/cutehackers/rail.git", tag: "v0.2.4"
  version "0.2.4"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-o", bin/"rail", "./cmd/rail"

    pkgshare.install "assets/skill"

    codex_skill_dir = prefix/"share/codex/skills/rail"
    codex_skill_dir.mkpath
    cp_r (pkgshare/"skill/Rail").children, codex_skill_dir
  end

  def caveats
    <<~EOS
      Rail installs its packaged Codex skill assets under:
        #{opt_pkgshare}/skill/Rail

      A prefix-local Codex-facing copy is also installed under:
        #{opt_prefix}/share/codex/skills/rail
    EOS
  end

  test do
    assert_match "compose-request", shell_output("#{bin}/rail compose-request 2>&1", 1)
    assert_predicate pkgshare/"skill"/"Rail"/"SKILL.md", :exist?
    assert_predicate prefix/"share/codex/skills/rail"/"SKILL.md", :exist?
  end
end
