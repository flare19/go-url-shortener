import { NavLink } from "react-router-dom";

function linkClass({ isActive }: { isActive: boolean }) {
  return `px-3 py-1.5 font-mono text-sm rounded transition-colors ${
    isActive ? "text-accent" : "text-zinc-400 hover:text-zinc-200"
  }`;
}

export default function Nav() {
  return (
    <nav className="flex items-center gap-2 border-b border-edge px-6 py-4">
      <span className="font-mono text-accent text-sm mr-4">$ url-shortener</span>
      <NavLink to="/" end className={linkClass}>
        shorten
      </NavLink>
      <NavLink to="/stats" className={linkClass}>
        stats
      </NavLink>
    </nav>
  );
}