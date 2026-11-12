import { useEffect, useRef, type InputHTMLAttributes } from 'react';

interface Props extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  /** 半选态，仅在 checked 为 false 时有意义。 */
  indeterminate?: boolean;
}

/** 原生复选框，额外支持半选态（通过 ref.indeterminate 设置）。 */
export function IndeterminateCheckbox({ indeterminate = false, ...props }: Props) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return <input ref={ref} type="checkbox" {...props} />;
}
