import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";

const HomePage = () => {
  const navigate = useNavigate();
  const [owner, setOwner] = useState("");
  const [repo, setRepo] = useState("");
  const [startDate, setStartDate] = useState(null);
  const [endDate, setEndDate] = useState(null);

  const submit = useCallback(async (event) => {
    event.preventDefault();
    navigate("/graph?owner=" + owner + "&repo=" + repo + "&start=" + startDate + "&end=" + endDate);
  }, [owner, repo, startDate, endDate, navigate]);

  return (
    <div>
      <form onSubmit={submit}>
        <div>
          <label htmlFor="owner">Who is the repo owner</label>
          <input
            autoFocus
            type="text"
            name="owner"
            id="owner"
            onChange={(event) => setOwner(event.target.value)}
          />
        </div>

        <div>
          <label htmlFor="repo">Who is the repo repo</label>
          <input
            autoFocus
            type="text"
            name="repo"
            id="repo"
            onChange={(event) => setRepo(event.target.value)}
          />
        </div>

        <div>
          <label htmlFor="start">Pick the start date for your graph</label>
          <input
            autoFocus
            type="date"
            name="start"
            id="start"
            onChange={(event) => setStartDate(event.target.value)}
          />
        </div>

        <div>
          <label htmlFor="end">Pick the end date for your graph</label>
          <input
            autoFocus
            type="date"
            name="end"
            id="end"
            onChange={(event) => setEndDate(event.target.value)}
          />
        </div>

        <button
          type="submit"
          className="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded">
          Show me the graph
        </button>

      </form>
    </div>
  );
};

export default HomePage;
