import { useParams, useSearchParams } from 'react-router-dom';
import useSWR from 'swr';
import HighlightGraph from '../components/HighlightGraph';

const fetcher = async (url) => {
  const res = await fetch(url);
  const data = await res.json();
  return data;
};

const GraphPage = () => {
  const [params] = useSearchParams();
  const owner = params.get('owner');
  const repo = params.get('repo');
  const start = params.get('start');
  const end = params.get('end');

  let url = `http://localhost:8080/graph?owner=${owner}&repo=${repo}`;
  if (start) {
    url += `&start=${start}`;
  }
  if (end) {
    url += `&end=${end}`;
  }

  const { data, error } = useSWR(url, fetcher);

  if (error) return <div>failed to load</div>;
  if (!data) return <div>loading...</div>;

  return (
    <>
      <HighlightGraph data={data} />
    </>
  );
}

export default GraphPage;
